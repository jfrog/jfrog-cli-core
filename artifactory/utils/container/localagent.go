package container

import (
	"fmt"
	"net/http"
	"path"
	"strings"

	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// Build-info builder for local agents tools such as: Docker or Podman.
type localAgentbuildInfoBuilder struct {
	buildInfoBuilder *buildInfoBuilder
	// Name of the container CLI tool e.g. docker
	containerManager ContainerManager
	commandType      CommandType
}

// Create new build info builder container CLI tool
func NewLocalAgentBuildInfoBuilder(image *Image, repository, buildName, buildNumber, project string, serviceManager artifactory.ArtifactoryServicesManager, commandType CommandType, containerManager ContainerManager) (*localAgentbuildInfoBuilder, error) {
	imageSha2, err := containerManager.Id(image)
	if err != nil {
		return nil, err
	}
	builder, err := newBuildInfoBuilder(image, repository, buildName, buildNumber, project, serviceManager)
	if err != nil {
		return nil, err
	}
	builder.setImageSha2(imageSha2)
	return &localAgentbuildInfoBuilder{
		buildInfoBuilder: builder,
		containerManager: containerManager,
		commandType:      commandType,
	}, err
}

func (labib *localAgentbuildInfoBuilder) GetLayers() *[]utils.ResultItem {
	return &labib.buildInfoBuilder.imageLayers
}

func (labib *localAgentbuildInfoBuilder) SetSkipTaggingLayers(skipTaggingLayers bool) {
	labib.buildInfoBuilder.skipTaggingLayers = skipTaggingLayers
}

// Create build-info for a docker image.
func (labib *localAgentbuildInfoBuilder) Build(module string) (*buildinfo.BuildInfo, error) {
	// Search for image build-info.
	candidateLayers, manifest, err := labib.searchImage()
	if err != nil {
		log.Warn(`Failed to collect build-info, couldn't find image "` + labib.buildInfoBuilder.image.name + `" in Artifactory`)
	} else {
		log.Debug("Found manifest.json. Proceeding to create build-info.")
	}
	// Create build-info from search results.
	return labib.buildInfoBuilder.createBuildInfo(labib.commandType, manifest, candidateLayers, module)
}

// Search an image in Artifactory and validate its sha2 with local image.
func (labib *localAgentbuildInfoBuilder) searchImage() (map[string]*utils.ResultItem, *manifest, error) {
	imagePath, err := labib.buildInfoBuilder.image.GetPath()
	if err != nil {
		return nil, nil, err
	}
	manifestPathsCandidates := getManifestPaths(imagePath, labib.buildInfoBuilder.getSearchableRepo(), labib.commandType)
	log.Debug("Start searching for image manifest.json")
	for _, path := range manifestPathsCandidates {
		log.Debug(`Searching in:"` + path + `"`)
		resultMap, err := labib.search(path)
		if err != nil {
			return nil, nil, err
		}
		manifest, err := getManifest(resultMap, labib.buildInfoBuilder.serviceManager, labib.buildInfoBuilder.repositoryDetails.key)
		if err != nil {
			return nil, nil, err
		}
		if manifest != nil && labib.isVerifiedManifest(manifest) {
			return resultMap, manifest, nil
		}
	}
	return nil, nil, errorutils.CheckErrorf(imageNotFoundErrorMessage, labib.buildInfoBuilder.image.name)
}

// Search image layers in artifactory by the provided image path in artifactory.
// If fat-manifest is found, use it to find our image in Artifactory.
func (labib *localAgentbuildInfoBuilder) search(imagePathPattern string) (resultMap map[string]*utils.ResultItem, err error) {
	resultMap, err = performSearch(imagePathPattern, labib.buildInfoBuilder.serviceManager)
	if err != nil {
		return
	}
	// Validate there are no .marker layers.
	totalDownloaded, err := downloadMarkerLayersToRemoteCache(resultMap, labib.buildInfoBuilder)
	if err != nil {
		return nil, err
	}
	if totalDownloaded > 0 {
		// Search again after .marker layer were downloaded.
		if resultMap, err = performSearch(imagePathPattern, labib.buildInfoBuilder.serviceManager); err != nil {
			return
		}
	}
	// Check if search results contain multi-architecture images (fat-manifest).
	if searchResult, ok := resultMap["list.manifest.json"]; labib.commandType == Pull && ok {
		// In case of a fat-manifest, Artifactory will create two folders.
		// One folder named as the image tag, which contains the fat manifest.
		// The second folder, named as image's manifest digest, contains the image layers and the image's manifest.
		log.Debug("Found list.manifest.json (fat-manifest). Searching for the image manifest digest in list.manifest.json")
		var digest string
		digest, err = labib.getImageDigestFromFatManifest(*searchResult)
		if err == nil && digest != "" {
			// Remove tag from pattern, place the manifest digest instead.
			imagePathPattern = strings.Replace(imagePathPattern, "/*", "", 1)
			imagePathPattern = path.Join(imagePathPattern[:strings.LastIndex(imagePathPattern, "/")], strings.Replace(digest, ":", "__", 1), "*")
			// Retry search.
			return labib.search(imagePathPattern)
		}
		log.Debug("Couldn't find matching digest in list.manifest.json")
	}
	return resultMap, err
}

// Verify manifest by comparing sha256, which references to the image digest. If there is no match, return nil.
func (labib *localAgentbuildInfoBuilder) isVerifiedManifest(imageManifest *manifest) bool {
	if imageManifest.Config.Digest != labib.buildInfoBuilder.imageSha2 {
		log.Debug(`Found incorrect manifest.json file. Expects digest "` + labib.buildInfoBuilder.imageSha2 + `" found "` + imageManifest.Config.Digest)
		return false
	}
	return true
}

func (labib *localAgentbuildInfoBuilder) getImageDigestFromFatManifest(fatManifest utils.ResultItem) (string, error) {
	var fatManifestContent *FatManifest
	if err := downloadLayer(fatManifest, &fatManifestContent, labib.buildInfoBuilder.serviceManager, labib.buildInfoBuilder.repositoryDetails.key); err != nil {
		log.Debug(`failed to unmarshal fat-manifest`)
		return "", err
	}
	imageOs, imageArch, err := labib.containerManager.OsCompatibility(labib.buildInfoBuilder.image)
	if err != nil {
		return "", err
	}
	return searchManifestDigest(imageOs, imageArch, fatManifestContent.Manifests), nil
}

// When a client tries to pull an image from a remote repository in Artifactory and the client has some the layers cached locally on the disk,
// then Artifactory will not download these layers into the remote repository cache. Instead, it will mark the layer artifacts with .marker suffix files in the remote cache.
// This function download all the marker layers into the remote cache repository.
func downloadMarkerLayersToRemoteCache(resultMap map[string]*utils.ResultItem, builder *buildInfoBuilder) (int, error) {
	if !builder.repositoryDetails.isRemote || len(resultMap) == 0 {
		return 0, nil
	}
	totalDownloaded := 0
	remoteRepo := builder.repositoryDetails.key
	imageName, err := builder.image.GetImageBaseName()
	if err != nil {
		return 0, err
	}
	clientDetails := builder.serviceManager.GetConfig().GetServiceDetails().CreateHttpClientDetails()
	// Search for marker layers
	for _, layerData := range resultMap {
		if strings.HasSuffix(layerData.Name, markerLayerSuffix) {
			log.Debug(fmt.Sprintf("Downloading %s layer into remote repository cache...", layerData.Name))
			baseUrl := builder.serviceManager.GetConfig().GetServiceDetails().GetUrl()
			endpoint := "api/docker/" + remoteRepo + "/v2/" + imageName + "/blobs/" + toNoneMarkerLayer(layerData.Name)
			resp, body, err := builder.serviceManager.Client().SendHead(baseUrl+endpoint, &clientDetails)
			if err != nil {
				return totalDownloaded, err
			}
			if resp.StatusCode != http.StatusOK {
				return totalDownloaded, errorutils.CheckErrorf("Artifactory response: " + resp.Status + "for" + string(body))
			}
			totalDownloaded++
		}
	}
	return totalDownloaded, nil
}

func handleMissingLayer(layerMediaType, layerFileName string) error {
	// Allow missing layer to be of a foreign type.
	if layerMediaType == foreignLayerMediaType {
		log.Info(fmt.Sprintf("Foreign layer: %s is missing in Artifactory and therefore will not be added to the build-info.", layerFileName))
		return nil
	}
	return errorutils.CheckErrorf("Could not find layer: " + layerFileName + " in Artifactory")
}
