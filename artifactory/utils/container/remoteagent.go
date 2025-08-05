package container

import (
	"strings"

	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// Build-info builder for remote agents tools such as: Kaniko, OpenShift CLI (oc), or buildx.
type RemoteAgentBuildInfoBuilder struct {
	buildInfoBuilder *buildInfoBuilder
	manifestSha2     string
}

func NewRemoteAgentBuildInfoBuilder(image *Image, repository, buildName, buildNumber, project string, serviceManager artifactory.ArtifactoryServicesManager, manifestSha256 string) (*RemoteAgentBuildInfoBuilder, error) {
	builder, err := newBuildInfoBuilder(image, repository, buildName, buildNumber, project, serviceManager)
	return &RemoteAgentBuildInfoBuilder{
		buildInfoBuilder: builder,
		manifestSha2:     manifestSha256,
	}, err
}

func (rabib *RemoteAgentBuildInfoBuilder) GetLayers() *[]utils.ResultItem {
	return &rabib.buildInfoBuilder.imageLayers
}

func (rabib *RemoteAgentBuildInfoBuilder) Build(module string) (*buildinfo.BuildInfo, error) {
	// Search for and image in Artifactory.
	results, err := rabib.searchImage()
	if err != nil {
		return nil, err
	}
	// Create build-info based on image manifest.
	if results["manifest.json"] != nil {
		searchResults, manifest, err := rabib.handleManifest(results)
		if err != nil {
			return nil, err
		}
		return rabib.buildInfoBuilder.createBuildInfo(Push, manifest, searchResults, module)
	}
	// Create build-info based on image fat-manifest.
	multiPlatformImages, fatManifestDetails, fatManifest, err := rabib.handleFatManifestImage(results)
	if err != nil {
		return nil, err
	}
	return rabib.buildInfoBuilder.createMultiPlatformBuildInfo(fatManifest, fatManifestDetails, multiPlatformImages, module)
}

// Search for image manifest and layers in Artifactory.
func (rabib *RemoteAgentBuildInfoBuilder) handleManifest(resultMap map[string]*utils.ResultItem) (map[string]*utils.ResultItem, *manifest, error) {
	if manifest, ok := resultMap["manifest.json"]; ok {
		if !rabib.isVerifiedManifest(manifest) {
			log.Debug("Manifest verification failed, continuing with SHA-based validation...")
		}
		manifest, err := getManifest(resultMap, rabib.buildInfoBuilder.serviceManager, rabib.buildInfoBuilder.repositoryDetails.key)
		if err != nil {
			return nil, nil, err
		}
		// Manifest may hold 'empty layers'. As a result, promotion will fail to promote the same layer more than once.
		rabib.buildInfoBuilder.imageSha2 = manifest.Config.Digest
		log.Debug("Found manifest.json. Proceeding to create build-info.")
		return resultMap, manifest, nil
	}
	return nil, nil, errorutils.CheckErrorf(`couldn't find image "%s" manifest in Artifactory`, rabib.buildInfoBuilder.image.name)
}

func (rabib *RemoteAgentBuildInfoBuilder) handleFatManifestImage(results map[string]*utils.ResultItem) (map[string][]*utils.ResultItem, *utils.ResultItem, *FatManifest, error) {
	if fatManifestResult, ok := results["list.manifest.json"]; ok {
		log.Debug("Found list.manifest.json. Proceeding to create build-info.")
		fatManifestRootPath := getFatManifestRoot(fatManifestResult.GetItemRelativeLocation()) + "/*"
		fatManifest, err := getFatManifest(results, rabib.buildInfoBuilder.serviceManager, rabib.buildInfoBuilder.repositoryDetails.key)
		if err != nil {
			return nil, nil, nil, err
		}
		multiPlatformImages, err := performMultiPlatformImageSearch(fatManifestRootPath, rabib.buildInfoBuilder.serviceManager)
		return multiPlatformImages, fatManifestResult, fatManifest, err
	}
	return nil, nil, nil, errorutils.CheckErrorf(`couldn't find image "%s" fat manifest in Artifactory`, rabib.buildInfoBuilder.image.name)
}

// Search image manifest or fat-manifest of and image.
func (rabib *RemoteAgentBuildInfoBuilder) searchImage() (resultMap map[string]*utils.ResultItem, err error) {
	longImageName, err := rabib.buildInfoBuilder.image.GetImageLongNameWithTag()
	if err != nil {
		return nil, err
	}
	imagePath := strings.Replace(longImageName, ":", "/", 1)

	// Search image's manifest.
	manifestPathsCandidates := getManifestPaths(imagePath, rabib.buildInfoBuilder.getSearchableRepo(), Push)
	log.Debug("Start searching for image manifest.json")

	// First try standard tag-based search
	for _, path := range manifestPathsCandidates {
		log.Debug(`Searching in:"` + path + `"`)
		resultMap, err = performSearch(path, rabib.buildInfoBuilder.serviceManager)
		if err != nil {
			return nil, err
		}
		if resultMap == nil {
			continue
		}
		if resultMap["list.manifest.json"] != nil || resultMap["manifest.json"] != nil {
			return resultMap, nil
		}
	}

	// If tag-based search failed and we have a SHA, try SHA-based search
	if rabib.manifestSha2 != "" {
		log.Debug("Tag-based search failed. Trying SHA-based search with: " + rabib.manifestSha2)
		// Extract repository path without tag
		repoPath := imagePath[:strings.LastIndex(imagePath, "/")]
		// Convert SHA format from sha256:xxx to sha256__xxx for Artifactory path format
		shaPath := strings.Replace(rabib.manifestSha2, ":", "__", 1)
		// Search for the image using SHA path
		shaSearchPath := repoPath + "/" + shaPath + "/*"
		log.Debug(`Searching by SHA in:"` + shaSearchPath + `"`)
		resultMap, err = performSearch(shaSearchPath, rabib.buildInfoBuilder.serviceManager)
		if err != nil {
			return nil, err
		}
		if resultMap != nil && (resultMap["list.manifest.json"] != nil || resultMap["manifest.json"] != nil) {
			log.Info("Found image by SHA digest in repository")
			return resultMap, nil
		}
	}

	return nil, errorutils.CheckErrorf(imageNotFoundErrorMessage, rabib.buildInfoBuilder.image.name)
}

// Verify manifest's sha256. Returns true if manifest is verified, false otherwise.
func (rabib *RemoteAgentBuildInfoBuilder) isVerifiedManifest(imageManifest *utils.ResultItem) bool {
	if imageManifest.GetProperty("docker.manifest.digest") != rabib.manifestSha2 {
		manifestDigest := imageManifest.GetProperty("docker.manifest.digest")
		log.Warn("Manifest digest mismatch detected. Local image digest: " + rabib.manifestSha2 + ", Repository digest: " + manifestDigest)
		log.Info("Proceeding with SHA-based validation to ensure correct image identification...")
		return false
	}
	return true
}

func getFatManifestRoot(fatManifestPath string) string {
	fatManifestPath = strings.TrimSuffix(fatManifestPath, "/")
	return fatManifestPath[:strings.LastIndex(fatManifestPath, "/")]
}
