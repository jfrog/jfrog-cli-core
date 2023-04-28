package utils

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	"github.com/jfrog/jfrog-client-go/http/jfroghttpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	// releasesRemoteEnv should be used for downloading the extractor jars through an Artifactory remote
	// repository, instead of downloading directly from releases.jfrog.io. The remote repository should be
	// configured to proxy releases.jfrog.io.
	// This env var should store a server ID and a remote repository in form of '<ServerID>/<RemoteRepo>'
	releasesRemoteEnv = "JFROG_CLI_RELEASES_REPO"
	// ExtractorsRemoteEnv is deprecated, it is replaced with releasesRemoteEnv.
	// Its functionality was similar to releasesRemoteEnv, but it proxies releases.jfrog.io/artifactory/oss-release-local instead.
	ExtractorsRemoteEnv = "JFROG_CLI_EXTRACTORS_REMOTE"
)

// Download the relevant build-info-extractor jar, if it does not already exist locally.
// By default, the jar is downloaded directly from jfrog releases.
// downloadPath: Artifactory download path.
// targetPath: The local download path (without the file name).
func DownloadExtractorIfNeeded(targetPath, downloadPath string) error {
	artDetails, remotePath, err := getExtractorsRemoteDetails(downloadPath)
	if err != nil {
		return err
	}

	return DownloadExtractor(artDetails, remotePath, targetPath)
}

// The getExtractorsRemoteDetails function is responsible for retrieving the server details necessary to download the build-info extractors.
// downloadPath - specifies the path in the remote repository from which the extractors will be downloaded.
func getExtractorsRemoteDetails(downloadPath string) (*config.ServerDetails, string, error) {
	releasesServerAndRepo := os.Getenv(releasesRemoteEnv)
	if releasesServerAndRepo != "" {
		return getRemoteDetails(releasesServerAndRepo, downloadPath, releasesRemoteEnv)
	}

	// Fallback to the deprecated JFROG_CLI_EXTRACTORS_REMOTE environment variable
	extractorsServerAndRepo := os.Getenv(ExtractorsRemoteEnv)
	if extractorsServerAndRepo != "" {
		return getRemoteDetails(extractorsServerAndRepo, downloadPath, ExtractorsRemoteEnv)
	}

	log.Info("The build-info-extractor jar is not cached locally. Downloading it now...\nYou can set the repository from which this jar is downloaded. Read more about it at https://www.jfrog.com/confluence/display/CLI/CLI+for+JFrog+Artifactory#CLIforJFrogArtifactory-DownloadingtheMavenandGradleExtractorJARs")
	log.Debug("'" + releasesRemoteEnv + "' environment variable is not configured. Downloading directly from releases.jfrog.io.")
	// If not configured to download through a remote repository in Artifactory, download from releases.jfrog.io.
	return &config.ServerDetails{ArtifactoryUrl: "https://releases.jfrog.io/artifactory/"}, path.Join("oss-release-local", downloadPath), nil
}

// getRemoteDetails function retrieve the server details and download path for the build-info extractor file.
// serverAndRepo - the server id and the remote repository that proxies releases.jfrog.io, in form of '<ServerID>/<RemoteRepo>'.
// downloadPath - specifies the path in the remote repository from which the extractors will be downloaded.
// remoteEnv - the relevant environment variable that was used: releasesRemoteEnv/ExtractorsRemoteEnv.
func getRemoteDetails(serverAndRepo, downloadPath, remoteEnv string) (*config.ServerDetails, string, error) {
	serverID, repoName, err := splitRepoAndServerId(serverAndRepo, remoteEnv)
	if err != nil {
		return nil, "", err
	}
	serverDetails, err := config.GetSpecificConfig(serverID, false, true)
	if err != nil {
		return nil, "", err
	}
	return serverDetails, getFullRemoteRepoPath(repoName, remoteEnv, downloadPath), err
}

func splitRepoAndServerId(serverAndRepo, remoteEnv string) (serverID string, repoName string, err error) {
	// The serverAndRepo is in form of '<ServerID>/<RemoteRepo>'
	lastSlashIndex := strings.LastIndex(serverAndRepo, "/")
	// Check that the format is valid
	invalidFormat := lastSlashIndex == -1 || lastSlashIndex == len(serverAndRepo)-1 || lastSlashIndex == 0
	if invalidFormat {
		return "", "", errorutils.CheckErrorf("'%s' environment variable is '%s' but should be '<server ID>/<repo name>'", remoteEnv, serverAndRepo)
	}
	serverID = serverAndRepo[:lastSlashIndex]
	repoName = serverAndRepo[lastSlashIndex+1:]
	return
}

func getFullRemoteRepoPath(repoName, remoteEnv, downloadPath string) string {
	if remoteEnv == releasesRemoteEnv {
		return path.Join(repoName, "artifactory", "oss-release-local", downloadPath)
	}
	return path.Join(repoName, downloadPath)
}

func DownloadExtractor(artDetails *config.ServerDetails, downloadPath, targetPath string) error {
	downloadUrl := fmt.Sprintf("%s%s", artDetails.ArtifactoryUrl, downloadPath)
	log.Info("Downloading build-info-extractor from", downloadUrl)
	filename, localDir := fileutils.GetFileAndDirFromPath(targetPath)

	downloadFileDetails := &httpclient.DownloadFileDetails{
		FileName:      filename,
		DownloadPath:  downloadUrl,
		LocalPath:     localDir,
		LocalFileName: filename,
	}

	auth, err := artDetails.CreateArtAuthConfig()
	if err != nil {
		return err
	}
	certsPath, err := coreutils.GetJfrogCertsDir()
	if err != nil {
		return err
	}

	client, err := jfroghttpclient.JfrogClientBuilder().
		SetCertificatesPath(certsPath).
		SetInsecureTls(artDetails.InsecureTls).
		SetClientCertPath(auth.GetClientCertPath()).
		SetClientCertKeyPath(auth.GetClientCertKeyPath()).
		AppendPreRequestInterceptor(auth.RunPreRequestFunctions).
		Build()
	if err != nil {
		return err
	}

	httpClientDetails := auth.CreateHttpClientDetails()
	resp, err := client.DownloadFile(downloadFileDetails, "", &httpClientDetails, false, false)
	if err == nil && resp.StatusCode != http.StatusOK {
		err = errorutils.CheckErrorf(resp.Status + " received when attempting to download " + downloadUrl)
	}

	return err
}
