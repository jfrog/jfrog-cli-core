package container

import (
	"fmt"
	buildinfo "github.com/jfrog/build-info-go/entities"
	"io/ioutil"
	"net/http"
	"path"
	"strings"

	artutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	Pull                      CommandType = "pull"
	Push                      CommandType = "push"
	foreignLayerMediaType     string      = "application/vnd.docker.image.rootfs.foreign.diff.tar.gzip"
	imageNotFoundErrorMessage string      = "Could not find docker image in Artifactory, expecting image tag: %s"
	markerLayerSuffix         string      = ".marker"
)

// Docker image build info builder.
type Builder interface {
	Build(module string) (*buildinfo.BuildInfo, error)
	UpdateArtifactsAndDependencies() error
	GetLayers() *[]utils.ResultItem
}

type buildInfoBuilder struct {
	image             *Image
	repositoryDetails RepositoryDetails
	buildName         string
	buildNumber       string
	project           string
	serviceManager    artifactory.ArtifactoryServicesManager

	// For Docker and Podman builds
	containerManager ContainerManager

	// For Kaniko and OpenShift CLI (oc) builds
	manifestSha256 string

	// internal fields
	imageId      string
	layers       []utils.ResultItem
	artifacts    []buildinfo.Artifact
	dependencies []buildinfo.Dependency
	commandType  CommandType
}

func NewBuildInfoBuilderForDockerOrPodman(image *Image, repository, buildName, buildNumber, project string, serviceManager artifactory.ArtifactoryServicesManager, commandType CommandType, containerManager ContainerManager) (Builder, error) {
	builder, err := newBuildInfoBuilder(image, repository, buildName, buildNumber, project, serviceManager, commandType)
	if err != nil {
		return nil, err
	}
	builder.containerManager = containerManager
	builder.imageId, err = builder.containerManager.Id(builder.image)
	return builder, err
}

func NewBuildInfoBuilderForKanikoOrOpenShift(image *Image, repository, buildName, buildNumber, project string, serviceManager artifactory.ArtifactoryServicesManager, commandType CommandType, manifestSha256 string) (Builder, error) {
	builder, err := newBuildInfoBuilder(image, repository, buildName, buildNumber, project, serviceManager, commandType)
	if err != nil {
		return nil, err
	}
	builder.manifestSha256 = manifestSha256
	return builder, err
}

// Create instance of docker build info builder.
func newBuildInfoBuilder(image *Image, repository, buildName, buildNumber, project string, serviceManager artifactory.ArtifactoryServicesManager, commandType CommandType) (*buildInfoBuilder, error) {
	var err error
	builder := &buildInfoBuilder{}
	builder.repositoryDetails.key = repository
	builder.repositoryDetails.isRemote, err = artutils.IsRemoteRepo(repository, serviceManager)
	if err != nil {
		return nil, err
	}
	builder.image = image
	builder.buildName = buildName
	builder.buildNumber = buildNumber
	builder.project = project
	builder.serviceManager = serviceManager
	builder.commandType = commandType
	return builder, nil
}

type RepositoryDetails struct {
	key      string
	isRemote bool
}

func (builder *buildInfoBuilder) GetLayers() *[]utils.ResultItem {
	return &builder.layers
}

// Create build info for a docker image.
func (builder *buildInfoBuilder) Build(module string) (*buildinfo.BuildInfo, error) {
	if err := builder.UpdateArtifactsAndDependencies(); err != nil {
		log.Warn(`Failed to collect build-info, couldn't find image "` + builder.image.tag + `" in Artifactory`)
		// Don't generate an empty build-info for build-docker-create and oc start-build if the image manifest was not found in Artifactory.
		if builder.containerManager == nil {
			return nil, err
		} else {
			log.Error("Failed populating the build-info module with docker artifacts and dependencies. Reason: " + err.Error())
		}
	}
	// Set build properties only when pushing image.
	if builder.commandType == Push {
		if _, err := builder.setBuildProperties(); err != nil {
			return nil, err
		}
	}
	return builder.createBuildInfo(module)
}

func (builder *buildInfoBuilder) getSearchableRepo() string {
	if builder.repositoryDetails.isRemote {
		return builder.repositoryDetails.key + "-cache"
	}
	return builder.repositoryDetails.key
}

// Search, validate and create image's artifacts and dependencies.
func (builder *buildInfoBuilder) UpdateArtifactsAndDependencies() error {
	// Search for image's manifest and layers.
	manifestLayers, manifestContent, err := builder.getManifestAndLayersDetails()
	if err != nil {
		return err
	}
	log.Debug("Found manifest.json. Proceeding to collect build-info.")
	// Manifest may hold 'empty layers'. As a result, promotion will fail to promote the same layer more than once.
	manifestContent.Layers = removeDuplicateLayers(manifestContent.Layers)
	manifestArtifact, manifestDependency := getManifestArtifact(manifestLayers), getManifestDependency(manifestLayers)
	configLayer, configLayerArtifact, configLayerDependency, err := builder.getConfigLayer(manifestLayers)
	if err != nil {
		return err
	}
	if builder.commandType == Push {
		return builder.handlePush(manifestArtifact, configLayerArtifact, manifestContent, configLayer, manifestLayers)
	}
	return builder.handlePull(manifestDependency, configLayerDependency, manifestContent, manifestLayers)
}

// Return all the search patterns in which manifest can be found.
func getManifestPaths(imagePath, repo string, commandType CommandType) []string {
	// pattern 1: reverse proxy e.g. ecosysjfrog-docker-local.jfrog.io.
	paths := []string{path.Join(repo, imagePath, "*")}
	// pattern 2: proxy-less e.g. orgab.jfrog.team/docker-local.
	endOfRepoNameIndex := strings.Index(imagePath[1:], "/")
	proxylessTag := imagePath[endOfRepoNameIndex+1:]
	paths = append(paths, path.Join(repo, proxylessTag, "*"))
	// If image path includes more than 3 slashes, Artifactory doesn't store this image under 'library', thus we should not look further.
	if commandType != Push && strings.Count(imagePath, "/") <= 3 {
		// pattern 3: reverse proxy - this time with 'library' as part of the path.
		paths = append(paths, path.Join(repo, "library", imagePath, "*"))
		// pattern 4: Assume proxy-less - this time with 'library' as part of the path.
		paths = append(paths, path.Join(repo, "library", proxylessTag, "*"))
	}
	return paths
}

// Search for image manifest and layers in Artifactory.
func (builder *buildInfoBuilder) getManifestAndLayersDetails() (layers map[string]*utils.ResultItem, manifestContent *manifest, err error) {
	imagePath, err := builder.image.Path()
	if err != nil {
		return nil, nil, err
	}
	manifestPathsCandidates := getManifestPaths(imagePath, builder.getSearchableRepo(), builder.commandType)
	log.Debug("Start searching for image manifest.json")
	for _, path := range manifestPathsCandidates {
		log.Debug(`Searching in:"` + path + `"`)
		layers, manifestContent, err = searchManifestAndLayersDetails(builder, path)
		if err != nil || manifestContent != nil {
			return layers, manifestContent, err
		}
	}
	return nil, nil, errorutils.CheckErrorf(imageNotFoundErrorMessage, builder.image.tag)
}

func (builder *buildInfoBuilder) handlePull(manifestDependency, configLayerDependency buildinfo.Dependency, imageManifest *manifest, searchResults map[string]*utils.ResultItem) error {
	// Add dependencies.
	builder.dependencies = append(builder.dependencies, manifestDependency)
	builder.dependencies = append(builder.dependencies, configLayerDependency)
	// Add image layers as dependencies.
	for i := 0; i < len(imageManifest.Layers); i++ {
		layerFileName := digestToLayer(imageManifest.Layers[i].Digest)
		item, layerExists := searchResults[layerFileName]
		if !layerExists {
			err := builder.handleMissingLayer(imageManifest.Layers[i].MediaType, layerFileName)
			if err != nil {
				return err
			}
			continue
		}
		builder.dependencies = append(builder.dependencies, item.ToDependency())
	}
	return nil
}

func (builder *buildInfoBuilder) handlePush(manifestArtifact, configLayerArtifact buildinfo.Artifact, imageManifest *manifest, configurationLayer *configLayer, searchResults map[string]*utils.ResultItem) error {
	// Add artifacts.
	builder.artifacts = append(builder.artifacts, manifestArtifact)
	builder.artifacts = append(builder.artifacts, configLayerArtifact)
	// Add layers.
	builder.layers = append(builder.layers, *searchResults["manifest.json"])
	builder.layers = append(builder.layers, *searchResults[digestToLayer(builder.imageId)])
	totalLayers := len(imageManifest.Layers)
	totalDependencies := configurationLayer.getNumberOfDependentLayers()
	// Add image layers as artifacts and dependencies.
	for i := 0; i < totalLayers; i++ {
		layerFileName := digestToLayer(imageManifest.Layers[i].Digest)
		item, layerExists := searchResults[layerFileName]
		if !layerExists {
			err := builder.handleMissingLayer(imageManifest.Layers[i].MediaType, layerFileName)
			if err != nil {
				return err
			}
			continue
		}
		// Decide if the layer is also a dependency.
		if i < totalDependencies {
			builder.dependencies = append(builder.dependencies, item.ToDependency())
		}
		builder.artifacts = append(builder.artifacts, item.ToArtifact())
		builder.layers = append(builder.layers, *item)
	}
	return nil
}

func (builder *buildInfoBuilder) handleMissingLayer(layerMediaType, layerFileName string) error {
	// Allow missing layer to be of a foreign type.
	if layerMediaType == foreignLayerMediaType {
		log.Info(fmt.Sprintf("Foreign layer: %s is missing in Artifactory and therefore will not be added to the build-info.", layerFileName))
		return nil
	}
	return errorutils.CheckErrorf("Could not find layer: " + layerFileName + " in Artifactory")
}

// Set build properties on image layers in Artifactory.
func (builder *buildInfoBuilder) setBuildProperties() (int, error) {
	props, err := artutils.CreateBuildProperties(builder.buildName, builder.buildNumber, builder.project)
	if err != nil {
		return 0, err
	}
	pathToFile, err := writeLayersToFile(builder.layers)
	if err != nil {
		return 0, err
	}
	reader := content.NewContentReader(pathToFile, content.DefaultKey)
	defer reader.Close()
	return builder.serviceManager.SetProps(services.PropsParams{Reader: reader, Props: props})
}

// Download the content of layer search result.
func (builder *buildInfoBuilder) downloadLayer(searchResult utils.ResultItem, result interface{}) error {
	// Search results may include artifacts from the remote-cache repository.
	// When artifact is expired, it cannot be downloaded from the remote-cache.
	// To solve this, change back the search results' repository, to its origin remote/virtual.
	searchResult.Repo = builder.repositoryDetails.key
	path := searchResult.GetItemRelativePath()
	return artutils.RemoteUnmarshal(builder.serviceManager, path, result)
}

func writeLayersToFile(layers []utils.ResultItem) (filePath string, err error) {
	writer, err := content.NewContentWriter("results", true, false)
	if err != nil {
		return
	}
	defer func() {
		err = writer.Close()
	}()
	for _, layer := range layers {
		writer.Write(layer)
	}
	filePath = writer.GetFilePath()
	return
}

// Create a docker build info.
func (builder *buildInfoBuilder) createBuildInfo(module string) (*buildinfo.BuildInfo, error) {
	imageProperties := map[string]string{}
	imageProperties["docker.image.id"] = builder.imageId
	imageProperties["docker.image.tag"] = builder.image.Tag()
	if module == "" {
		imageName, err := builder.image.Name()
		if err != nil {
			return nil, err
		}
		module = imageName
	}
	buildInfo := &buildinfo.BuildInfo{Modules: []buildinfo.Module{{
		Id:           module,
		Type:         buildinfo.Docker,
		Properties:   imageProperties,
		Artifacts:    builder.artifacts,
		Dependencies: builder.dependencies,
	}}}
	return buildInfo, nil
}

// Return - manifest artifacts as buildinfo.Artifact struct.
func getManifestArtifact(searchResults map[string]*utils.ResultItem) (artifact buildinfo.Artifact) {
	item := searchResults["manifest.json"]
	return buildinfo.Artifact{Name: "manifest.json", Type: "json", Checksum: &buildinfo.Checksum{Sha1: item.Actual_Sha1, Md5: item.Actual_Md5}, Path: path.Join(item.Repo, item.Path, item.Name)}
}

// Return - manifest dependency as buildinfo.Dependency struct.
func getManifestDependency(searchResults map[string]*utils.ResultItem) (dependency buildinfo.Dependency) {
	item := searchResults["manifest.json"]
	return buildinfo.Dependency{Id: "manifest.json", Type: "json", Checksum: &buildinfo.Checksum{Sha1: item.Actual_Sha1, Md5: item.Actual_Md5}}
}

// Download and read the config layer from Artifactory.
// Returned values:
// configurationLayer - pointer to the configuration layer struct, retrieved from Artifactory.
// artifact - configuration layer as buildinfo.Artifact struct.
// dependency - configuration layer as buildinfo.Dependency struct.
func (builder *buildInfoBuilder) getConfigLayer(searchResults map[string]*utils.ResultItem) (configurationLayer *configLayer, artifact buildinfo.Artifact, dependency buildinfo.Dependency, err error) {
	item := searchResults[digestToLayer(builder.imageId)]
	configurationLayer = new(configLayer)
	if err := builder.downloadLayer(*item, &configurationLayer); err != nil {
		return nil, buildinfo.Artifact{}, buildinfo.Dependency{}, err
	}
	artifact = buildinfo.Artifact{Name: digestToLayer(builder.imageId), Checksum: &buildinfo.Checksum{Sha1: item.Actual_Sha1, Md5: item.Actual_Md5}, Path: path.Join(item.Repo, item.Path, item.Name)}
	dependency = buildinfo.Dependency{Id: digestToLayer(builder.imageId), Checksum: &buildinfo.Checksum{Sha1: item.Actual_Sha1, Md5: item.Actual_Md5}}
	return
}

// Search for manifest in Artifactory, If not found, returns 'manifestContent' as nil.
func searchManifestAndLayersDetails(builder *buildInfoBuilder, imagePathPattern string) (resultMap map[string]*utils.ResultItem, manifestContent *manifest, err error) {
	resultMap, err = searchHandler(imagePathPattern, builder)
	if err != nil || len(resultMap) == 0 {
		log.Debug("Couldn't find manifest.json. Image path pattern: ", imagePathPattern, ".")
		return
	}
	// Check if search results contain manifest.json
	searchResult, ok := resultMap["manifest.json"]
	if ok {
		// Found a manifest. Verify manifest is the same as the builder image.
		if builder.containerManager == nil {
			manifestContent, err = verifyManifestBySha256(*searchResult, builder)
		} else {
			manifestContent, err = verifyManifestByDigest(*searchResult, builder)
		}
	} else {
		if builder.containerManager == nil {
			err = errorutils.CheckErrorf("build info collection for multi-architecture images is not supported in build-docker-create and oc start-build commands")
			return
		}
		// Check if search results contain multi-architecture images (fat-manifest).
		if searchResult, ok := resultMap["list.manifest.json"]; ok {
			// In case of a fat-manifest, Artifactory will create two folders.
			// One folder named as the image tag, which contains the fat manifest.
			// The second folder, named as image's manifest digest, contains the image layers and the image's manifest.
			log.Debug("Found list.manifest.json (fat-manifest). Searching for the image manifest digest in list.manifest.json")
			var digest string
			digest, err = getImageDigestFromFatManifest(*searchResult, builder)
			if err == nil && digest != "" {
				// Remove tag from pattern, place the manifest digest instead.
				imagePathPattern = strings.Replace(imagePathPattern, "/*", "", 1)
				imagePathPattern = path.Join(imagePathPattern[:strings.LastIndex(imagePathPattern, "/")], strings.Replace(digest, ":", "__", 1), "*")
				// Retry search.
				return searchManifestAndLayersDetails(builder, imagePathPattern)
			}
			log.Debug("Couldn't find matching digest in list.manifest.json")
		}
	}
	return
}

func getImageDigestFromFatManifest(fatManifest utils.ResultItem, builder *buildInfoBuilder) (string, error) {
	var fatManifestContent *FatManifest
	if err := builder.downloadLayer(fatManifest, &fatManifestContent); err != nil {
		log.Debug(`failed to unmarshal fat-manifest`)
		return "", err
	}
	imageOs, imageArch, err := builder.containerManager.OsCompatibility(builder.image)
	if err != nil {
		return "", err
	}
	return searchManifestDigest(imageOs, imageArch, fatManifestContent.Manifests), nil
}

// Verify manifest contains the builder image digest. If there is no match, return nil.
func verifyManifestBySha256(manifestSearchResult utils.ResultItem, builder *buildInfoBuilder) (imageManifest *manifest, err error) {
	if manifestSearchResult.GetProperty("docker.manifest.digest") != builder.manifestSha256 {
		log.Debug(`Found incorrect manifest.json file. Expects sha256 "` + builder.manifestSha256 + `" found "` + manifestSearchResult.GetProperty("sha256"))
		return
	}
	log.Debug(`Found manifest.json with expected sha256: "` + builder.manifestSha256)
	if err = builder.downloadLayer(manifestSearchResult, &imageManifest); err != nil || imageManifest == nil {
		return
	}
	builder.imageId = imageManifest.Config.Digest
	return
}

// Verify manifest by comparing config digest, which references to the image digest. If there is no match, return nil.
func verifyManifestByDigest(manifestSearchResult utils.ResultItem, builder *buildInfoBuilder) (imageManifest *manifest, err error) {
	if err = builder.downloadLayer(manifestSearchResult, &imageManifest); err != nil {
		return
	}
	if imageManifest.Config.Digest != builder.imageId {
		log.Debug(`Found incorrect manifest.json file. Expects digest "` + builder.imageId + `" found "` + imageManifest.Config.Digest)
		imageManifest = nil
	}
	return
}

// Read the file which contains the following format: 'IMAGE-TAG-IN-ARTIFACTORY'@sha256'SHA256-OF-THE-IMAGE-MANIFEST'.
func GetImageTagWithDigest(filePath string) (tag string, sha256 string, err error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Debug("ioutil.ReadFile failed with '%s'\n", err)
		err = errorutils.CheckError(err)
		return
	}
	splittedData := strings.Split(string(data), `@`)
	if len(splittedData) != 2 {
		err = errorutils.CheckErrorf(`unexpected file format "` + filePath + `". The file should include one line in the following format: image-tag@sha256`)
		return
	}
	tag, sha256 = splittedData[0], strings.Trim(splittedData[1], "\n")
	if tag == "" || sha256 == "" {
		err = errorutils.CheckErrorf(`missing image-tag/sha256 in file: "` + filePath + `"`)
	}
	return
}

// Search for manifest digest in fat manifest, which contains specific platforms.
func searchManifestDigest(imageOs, imageArch string, manifestList []ManifestDetails) (digest string) {
	for _, manifest := range manifestList {
		if manifest.Platform.Os == imageOs && manifest.Platform.Architecture == imageArch {
			digest = manifest.Digest
			break
		}
	}
	return
}

func searchHandler(imagePathPattern string, builder *buildInfoBuilder) (resultMap map[string]*utils.ResultItem, err error) {
	resultMap, err = performSearch(imagePathPattern, builder.serviceManager)
	if err != nil {
		return
	}
	// Validate there are no .marker layers.
	if totalDownloaded, err := downloadMarkerLayersToRemoteCache(resultMap, builder); err != nil || totalDownloaded == 0 {
		return resultMap, err
	}
	log.Debug("Marker layers were found, updating search results.")
	return performSearch(imagePathPattern, builder.serviceManager)
}

// return a map of: layer-digest -> layer-search-result
func performSearch(imagePathPattern string, serviceManager artifactory.ArtifactoryServicesManager) (resultMap map[string]*utils.ResultItem, err error) {
	searchParams := services.NewSearchParams()
	searchParams.CommonParams = &utils.CommonParams{}
	searchParams.Pattern = imagePathPattern
	reader, err := serviceManager.SearchFiles(searchParams)
	if err != nil {
		return nil, err
	}
	defer func() {
		if deferErr := reader.Close(); err == nil {
			err = deferErr
		}
	}()
	resultMap = map[string]*utils.ResultItem{}
	for resultItem := new(utils.ResultItem); reader.NextRecord(resultItem) == nil; resultItem = new(utils.ResultItem) {
		resultMap[resultItem.Name] = resultItem
	}
	return resultMap, reader.GetError()
}

// Digest of type sha256:30daa5c11544632449b01f450bebfef6b89644e9e683258ed05797abe7c32a6e to
// sha256__30daa5c11544632449b01f450bebfef6b89644e9e683258ed05797abe7c32a6e
func digestToLayer(digest string) string {
	return strings.Replace(digest, ":", "__", 1)
}

// Get the number of dependencies layers from the config.
func (configLayer *configLayer) getNumberOfDependentLayers() int {
	layersNum := len(configLayer.History)
	newImageLayers := true
	for i := len(configLayer.History) - 1; i >= 0; i-- {
		if newImageLayers {
			layersNum--
		}
		if !newImageLayers && configLayer.History[i].EmptyLayer {
			layersNum--
		}
		createdBy := configLayer.History[i].CreatedBy
		if strings.Contains(createdBy, "ENTRYPOINT") || strings.Contains(createdBy, "MAINTAINER") {
			newImageLayers = false
		}
	}
	return layersNum
}

func removeDuplicateLayers(imageMLayers []layer) []layer {
	res := imageMLayers[:0]
	// Use map to record duplicates as we find them.
	encountered := map[string]bool{}
	for _, v := range imageMLayers {
		if !encountered[v.Digest] {
			res = append(res, v)
			encountered[v.Digest] = true
		}
	}
	return res
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
	imageName := getImageName(builder.image.Tag())
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

func getImageName(image string) string {
	imageId, tag := strings.LastIndex(image, "/"), strings.LastIndex(image, ":")
	if imageId == -1 || tag == -1 {
		return ""
	}
	return image[imageId+1 : tag]
}

func toNoneMarkerLayer(layer string) string {
	imageId := strings.Replace(layer, "__", ":", 1)
	return strings.Replace(imageId, ".marker", "", 1)
}

// To unmarshal config layer file
type configLayer struct {
	History []history `json:"history,omitempty"`
}

type history struct {
	Created    string `json:"created,omitempty"`
	CreatedBy  string `json:"created_by,omitempty"`
	EmptyLayer bool   `json:"empty_layer,omitempty"`
}

// To unmarshal manifest.json file
type manifest struct {
	Config manifestConfig `json:"config,omitempty"`
	Layers []layer        `json:"layers,omitempty"`
}

type manifestConfig struct {
	Digest string `json:"digest,omitempty"`
}

type layer struct {
	Digest    string `json:"digest,omitempty"`
	MediaType string `json:"mediaType,omitempty"`
}

type CommandType string
