package dependencies

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	biutils "github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	extractorsMetadataPath  = "org/jfrog/buildinfo/%s/maven-metadata.xml"
	mavenExtractorArtifact  = "build-info-extractor-maven3"
	gradleExtractorArtifact = "build-info-extractor-gradle"
	versionsCacheDirName    = "extractor-versions"
	// The path of an extractor jar: the artifact and the version, twice.
	extractorJarPath = "org/jfrog/buildinfo/%[1]s/%[2]s/%[1]s-%[2]s-uber.jar"
)

// The lookup runs on every command. A request for the metadata file takes 1 to 4 seconds on a slow network,
// so the timeout must be long enough for it, and short enough to not block a command when the network is not available.
var extractorVersionsTimeout = 10 * time.Second

// How long the list of versions is used before it is read again. A new extractor version reaches the users within this time.
var extractorVersionsCacheTTL = time.Hour

// How long a failed lookup is not tried again. A command does not wait for a network that is not available on every run.
var extractorVersionsFailureTTL = 10 * time.Minute

// The timeout of the check that Curation still allows a jar that is already on disk. It is shorter than the lookup: the jar can be used without it.
var extractorVerifyTimeout = 5 * time.Second

// versionsCache is the list of versions that is saved between commands.
type versionsCache struct {
	// The URL of the metadata file. A list of another repository is not used.
	Source    string    `json:"source"`
	FetchedAt time.Time `json:"fetchedAt"`
	// The time of the last failed lookup. The lookup is not tried again for extractorVersionsFailureTTL.
	FailedAt time.Time `json:"failedAt,omitempty"`
	// All the stable versions, newest first. Empty if no lookup succeeded.
	Versions []string `json:"versions"`
}

// ListMavenExtractorVersions lists the versions of the Maven build-info extractor of the given major line, newest first.
// It matches build-info-go's utils.ExtractorVersionsFunc.
func ListMavenExtractorVersions(majorLine string) (biutils.ExtractorVersions, error) {
	return listExtractorVersions(mavenExtractorArtifact, coreutils.MavenExtractorVersionEnv, majorLine, false)
}

// ListGradleExtractorVersions lists the versions of the Gradle build-info extractor of the given major line, newest first.
// The init script of the CLI depends on the major line, so a pinned version must belong to it.
// It matches build-info-go's utils.ExtractorVersionsFunc.
func ListGradleExtractorVersions(majorLine string) (biutils.ExtractorVersions, error) {
	return listExtractorVersions(gradleExtractorArtifact, coreutils.GradleExtractorVersionEnv, majorLine, true)
}

func listExtractorVersions(artifact, pinEnv, majorLine string, pinMustMatchMajorLine bool) (biutils.ExtractorVersions, error) {
	if pinned := strings.TrimSpace(os.Getenv(pinEnv)); pinned != "" {
		if err := validatePinnedVersion(pinned, pinEnv, majorLine, pinMustMatchMajorLine); err != nil {
			return biutils.ExtractorVersions{}, err
		}
		log.Info(fmt.Sprintf("Using the build-info extractor version %s, as set by %s.", pinned, pinEnv))
		return biutils.ExtractorVersions{Versions: []string{pinned}, Pinned: true, PinEnv: pinEnv}, nil
	}
	versions, listIsCurrent, err := getAllExtractorVersions(artifact)
	if err != nil {
		return biutils.ExtractorVersions{PinEnv: pinEnv}, err
	}
	found := biutils.ExtractorVersions{Versions: biutils.FilterMajorLine(versions, majorLine), PinEnv: pinEnv}
	if listIsCurrent {
		// A jar on disk is not downloaded again, so the download cannot tell that Curation blocked it after the first download.
		found.Verify = func(version string) error { return verifyExtractor(artifact, version) }
	}
	return found, nil
}

func validatePinnedVersion(pinned, pinEnv, majorLine string, mustMatchMajorLine bool) error {
	if !biutils.IsStableVersion(pinned) {
		return errorutils.CheckErrorf("%w: invalid value %q of the %s environment variable. Expected a version such as 1.2.3", biutils.ErrInvalidExtractorVersion, pinned, pinEnv)
	}
	if mustMatchMajorLine && biutils.MajorLine(pinned) != majorLine {
		return errorutils.CheckErrorf("%w: the version %s set by the %s environment variable does not fit this Gradle version. The version must be %s.x", biutils.ErrInvalidExtractorVersion, pinned, pinEnv, majorLine)
	}
	return nil
}

// getAllExtractorVersions returns all the stable versions of an extractor, newest first.
// It uses the saved list if it is fresh. If the lookup fails, it uses an old saved list, which is better than no list.
// listIsCurrent is true when the list is fresh, from the saved file or from a fetch just made, so the server was reached now or not long ago.
// It is false when the list is old because the lookup failed: the server is not asked about anything else then.
func getAllExtractorVersions(artifact string) (versions []string, listIsCurrent bool, err error) {
	// The list comes from the same place the extractor jars are downloaded from:
	// the remote repository from the environment, or releases.jfrog.io.
	server, remotePath, err := getExtractorsRemoteDetails(fmt.Sprintf(extractorsMetadataPath, artifact), false)
	if err != nil {
		return nil, false, err
	}
	source := server.ArtifactoryUrl + remotePath
	cache := readVersionsCache(artifact, source)
	if cache != nil && time.Since(cache.FetchedAt) < extractorVersionsCacheTTL {
		log.Debug("Using the saved list of extractor versions from", cache.FetchedAt.Format(time.RFC3339))
		return cache.Versions, true, nil
	}
	if cache != nil && time.Since(cache.FailedAt) < extractorVersionsFailureTTL {
		log.Debug("The last lookup of the extractor versions failed at", cache.FailedAt.Format(time.RFC3339), "so it is not tried again yet.")
		versions, err = failedLookupResult(cache, nil)
		return versions, false, err
	}
	versions, err = fetchExtractorVersions(server, remotePath)
	if err == nil {
		writeVersionsCache(artifact, versionsCache{Source: source, FetchedAt: time.Now().UTC(), Versions: versions})
		return versions, true, nil
	}
	failed := versionsCache{Source: source, FailedAt: time.Now().UTC()}
	if cache != nil {
		failed.FetchedAt, failed.Versions = cache.FetchedAt, cache.Versions
	}
	writeVersionsCache(artifact, failed)
	versions, err = failedLookupResult(&failed, err)
	return versions, false, err
}

// verifyExtractor asks the server if Curation still allows a version of an extractor whose jar is already on disk.
// It returns an error that wraps biutils.ErrBlockedByCuration for a blocked version. Any other error means that the server could not tell.
// The answer is kept for extractorVersionsCacheTTL, like the list of versions, so most commands send no request.
func verifyExtractor(artifact, version string) error {
	downloadPath := fmt.Sprintf(extractorJarPath, artifact, version)
	server, remotePath, err := getExtractorsRemoteDetails(downloadPath, false)
	if err != nil {
		return err
	}
	url := server.ArtifactoryUrl + remotePath
	if err = checkRecentlyBlocked(url); err != nil {
		return err
	}
	if recentlyVerified(url) {
		return nil
	}
	client, httpClientDetails, err := createHttpClient(server, extractorVerifyTimeout)
	if err != nil {
		return err
	}
	log.Debug("Checking that the extractor", url, "is still allowed")
	// The same request that starts a download: it does not send the jar.
	if _, _, err = client.GetRemoteFileDetails(url, &httpClientDetails); err == nil {
		rememberVerified(url)
		return nil
	}
	if blockedErr := checkCurationBlock(client, httpClientDetails, url, err); blockedErr != nil {
		rememberBlocked(url)
		return blockedErr
	}
	return err
}

// failedLookupResult returns the old saved list, which is better than no list. Without one, it returns the error of the lookup.
func failedLookupResult(cache *versionsCache, lookupErr error) ([]string, error) {
	if len(cache.Versions) == 0 {
		if lookupErr == nil {
			lookupErr = fmt.Errorf("the last lookup failed at %s and is not tried again for %s", cache.FailedAt.Format(time.RFC3339), extractorVersionsFailureTTL)
		}
		return nil, lookupErr
	}
	if lookupErr != nil {
		log.Warn("Failed to look up the latest extractor version. Using the saved list from "+cache.FetchedAt.Format(time.RFC3339)+":", lookupErr.Error())
	}
	return cache.Versions, nil
}

func fetchExtractorVersions(server *config.ServerDetails, remotePath string) ([]string, error) {
	client, httpClientDetails, err := createHttpClient(server, extractorVersionsTimeout)
	if err != nil {
		return nil, err
	}
	url := server.ArtifactoryUrl + remotePath
	log.Debug("Looking up the extractor versions in", url)
	resp, metadata, _, err := client.SendGet(url, true, &httpClientDetails)
	if err != nil {
		return nil, err
	}
	if err = errorutils.CheckResponseStatusWithBody(resp, metadata, http.StatusOK); err != nil {
		return nil, err
	}
	versions, err := biutils.ParseExtractorVersions(metadata)
	return versions, errorutils.CheckError(err)
}

func versionsCachePath(artifact string) (string, error) {
	return urlCachePath(artifact + ".json")
}

// readVersionsCache returns the saved list, or nil if there is none for the source. A saved list is never an error: it is only a speed-up.
func readVersionsCache(artifact, source string) *versionsCache {
	cachePath, err := versionsCachePath(artifact)
	if err != nil {
		return nil
	}
	content, err := os.ReadFile(cachePath)
	if err != nil {
		return nil
	}
	cache := &versionsCache{}
	if err = json.Unmarshal(content, cache); err != nil || cache.Source != source {
		return nil
	}
	// The file can come from another job or user. Every version becomes a path and a URL, so it must look like a version.
	for _, version := range cache.Versions {
		if !biutils.IsStableVersion(version) {
			return nil
		}
	}
	return cache
}

func writeVersionsCache(artifact string, cache versionsCache) {
	content, err := json.Marshal(cache)
	if err != nil {
		return
	}
	cachePath, err := versionsCachePath(artifact)
	if err == nil {
		err = writeCacheFile(cachePath, content)
	}
	if err != nil {
		log.Debug("Failed to save the list of extractor versions:", err.Error())
	}
}

// writeCacheFile saves a cache file in a folder that only the user can use: these files decide which extractor jar runs.
func writeCacheFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return biutils.WriteFileAtomic(path, bytes.NewReader(content), 0600)
}
