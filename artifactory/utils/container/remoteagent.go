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
type remoteAgentbuildInfoBuilder struct {
	buildInfoBuilder *buildInfoBuilder
	manifestSha2     string
}

func NewRemoteAgentBuildInfoBuilder(image *Image, repository, buildName, buildNumber, project string, serviceManager artifactory.ArtifactoryServicesManager, manifestSha256 string) (*remoteAgentbuildInfoBuilder, error) {
	builder, err := newBuildInfoBuilder(image, repository, buildName, buildNumber, project, serviceManager)
	return &remoteAgentbuildInfoBuilder{
		buildInfoBuilder: builder,
		manifestSha2:     manifestSha256,
	}, err
}

func (rabib *remoteAgentbuildInfoBuilder) GetLayers() *[]utils.ResultItem {
	return &rabib.buildInfoBuilder.imageLayers
}

func (rabib *remoteAgentbuildInfoBuilder) Build(module string) (*buildinfo.BuildInfo, error) {
	// Search for and image in Artifactory.
	results, resultsPath, err := rabib.searchImage()
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
	multiPlatformImages, fatManifestDetails, fatManifest, err := rabib.handleFatManifestImage(results, resultsPath)
	if err != nil {
		return nil, err
	}
	return rabib.buildInfoBuilder.createMultiPlatformBuildInfo(fatManifest, fatManifestDetails, multiPlatformImages, module)
}

// Search for image manifest and layers in Artifactory.
func (rabib *remoteAgentbuildInfoBuilder) handleManifest(resultMap map[string]*utils.ResultItem) (map[string]*utils.ResultItem, *manifest, error) {
	if manifest, ok := resultMap["manifest.json"]; ok {
		err := rabib.isVerifiedManifest(manifest)
		if err != nil {
			return nil, nil, err

		}
		manifest, err := getManifest(resultMap, rabib.buildInfoBuilder.serviceManager, rabib.buildInfoBuilder.repositoryDetails.key)
		if err != nil {
			return nil, nil, err
		}
		// // Manifest may hold 'empty layers'. As a result, promotion will fail to promote the same layer more than once.
		rabib.buildInfoBuilder.imageSha2 = manifest.Config.Digest
		log.Debug("Found manifest.json. Proceeding to create build-info.")
		return resultMap, manifest, nil
	}
	return nil, nil, errorutils.CheckErrorf(`couldn't find image "` + rabib.buildInfoBuilder.image.name + `" manifest in Artifactory`)
}

func (rabib *remoteAgentbuildInfoBuilder) handleFatManifestImage(results map[string]*utils.ResultItem, resultPath string) (map[string][]*utils.ResultItem, *utils.ResultItem, *FatManifest, error) {
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
	return nil, nil, nil, errorutils.CheckErrorf(`couldn't find image "` + rabib.buildInfoBuilder.image.name + `" fat manifest in Artifactory`)
}

// Search image manifest or fat-manifest of and image.
func (rabib *remoteAgentbuildInfoBuilder) searchImage() (resultMap map[string]*utils.ResultItem, path string, err error) {
	imagePath, err := rabib.buildInfoBuilder.image.GetPath()
	if err != nil {
		return nil, "", err
	}
	// Search image's manifest.
	manifestPathsCandidates := getManifestPaths(imagePath, rabib.buildInfoBuilder.getSearchableRepo(), Push)
	log.Debug("Start searching for image manifest.json")
	for _, path := range manifestPathsCandidates {
		log.Debug(`Searching in:"` + path + `"`)
		resultMap, err := performSearch(path, rabib.buildInfoBuilder.serviceManager)
		if err != nil {
			return nil, "", err
		}
		if resultMap == nil {
			continue
		}
		if resultMap["list.manifest.json"] != nil || resultMap["manifest.json"] != nil {
			return resultMap, path, nil
		}
	}
	return nil, "", errorutils.CheckErrorf(imageNotFoundErrorMessage, rabib.buildInfoBuilder.image.name)
}

// Verify manifest's sha256. If there is no match, return nil.
func (rabib *remoteAgentbuildInfoBuilder) isVerifiedManifest(imageManifest *utils.ResultItem) error {
	if imageManifest.GetProperty("docker.manifest.digest") != rabib.manifestSha2 {
		return errorutils.CheckErrorf(`Found incorrect manifest.json file. Expects digest "` + rabib.manifestSha2 + `" found "` + imageManifest.GetProperty("docker.manifest.digest"))
	}
	return nil
}

func getFatManifestRoot(fatManifestPath string) string {
	fatManifestPath = strings.TrimSuffix(fatManifestPath, "/")
	return fatManifestPath[:strings.LastIndex(fatManifestPath, "/")]
}
