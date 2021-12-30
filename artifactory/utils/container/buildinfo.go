package container

import (
	"encoding/json"
	"io/ioutil"
	"path"
	"strings"

	buildinfo "github.com/jfrog/build-info-go/entities"

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
	imageSha2         string
	// If true, don't set layers props in Artifactory.
	skipTaggingLayers bool
	imageLayers       []utils.ResultItem
}

// Create instance of docker build info builder.
func newBuildInfoBuilder(image *Image, repository, buildName, buildNumber, project string, serviceManager artifactory.ArtifactoryServicesManager) (*buildInfoBuilder, error) {
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
	return builder, nil
}

type RepositoryDetails struct {
	key      string
	isRemote bool
}

func (builder *buildInfoBuilder) setImageSha2(imageSha2 string) {
	builder.imageSha2 = imageSha2
}

func (builder *buildInfoBuilder) setskipTaggingLayers(skipTaggingLayers bool) {
	builder.skipTaggingLayers = skipTaggingLayers
}

func (builder *buildInfoBuilder) GetLayers() *[]utils.ResultItem {
	return &builder.imageLayers
}

func (builder *buildInfoBuilder) getSearchableRepo() string {
	if builder.repositoryDetails.isRemote {
		return builder.repositoryDetails.key + "-cache"
	}
	return builder.repositoryDetails.key
}

// Set build properties on image layers in Artifactory.
func setBuildProperties(buildName, buildNumber, project string, imageLayers []utils.ResultItem, serviceManager artifactory.ArtifactoryServicesManager) error {
	props, err := artutils.CreateBuildProperties(buildName, buildNumber, project)
	if err != nil {
		return err
	}
	pathToFile, err := writeLayersToFile(imageLayers)
	if err != nil {
		return err
	}
	reader := content.NewContentReader(pathToFile, content.DefaultKey)
	defer reader.Close()
	_, err = serviceManager.SetProps(services.PropsParams{Reader: reader, Props: props})
	return err
}

// Download the content of layer search result.
func downloadLayer(searchResult utils.ResultItem, result interface{}, serviceManager artifactory.ArtifactoryServicesManager, repo string) error {
	// Search results may include artifacts from the remote-cache repository.
	// When artifact is expired, it cannot be downloaded from the remote-cache.
	// To solve this, change back the search results' repository, to its origin remote/virtual.
	searchResult.Repo = repo
	path := searchResult.GetItemRelativePath()
	return artutils.RemoteUnmarshal(serviceManager, path, result)
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

// Return - manifest artifacts as buildinfo.Artifact struct.
func getManifestArtifact(manifest *utils.ResultItem) (artifact buildinfo.Artifact) {
	return buildinfo.Artifact{Name: "manifest.json", Type: "json", Checksum: &buildinfo.Checksum{Sha1: manifest.Actual_Sha1, Md5: manifest.Actual_Md5}, Path: path.Join(manifest.Path, manifest.Name)}
}

// Return - fat manifest artifacts as buildinfo.Artifact struct.
func getFatManifestArtifact(fatManifest *utils.ResultItem) (artifact buildinfo.Artifact) {
	return buildinfo.Artifact{Name: "list.manifest.json", Type: "json", Checksum: &buildinfo.Checksum{Sha1: fatManifest.Actual_Sha1, Md5: fatManifest.Actual_Md5}, Path: path.Join(fatManifest.Path, fatManifest.Name)}
}

// Return - manifest dependency as buildinfo.Dependency struct.
func getManifestDependency(searchResults *utils.ResultItem) (dependency buildinfo.Dependency) {
	return buildinfo.Dependency{Id: "manifest.json", Type: "json", Checksum: &buildinfo.Checksum{Sha1: searchResults.Actual_Sha1, Md5: searchResults.Actual_Md5}}
}

// Read the file which contains the following format: 'IMAGE-TAG-IN-ARTIFACTORY'@sha256'SHA256-OF-THE-IMAGE-MANIFEST'.
func GetImageTagWithDigest(filePath string) (tag string, sha256 string, err error) {
	var buildxMetaData buildxMetaData
	var data []byte
	data, err = ioutil.ReadFile(filePath)
	if errorutils.CheckError(err) != nil {
		log.Debug("ioutil.ReadFile failed with '%s'\n", err)
		return
	}
	json.Unmarshal(data, &buildxMetaData)
	// Try to read buildx metadata file.
	if buildxMetaData.ImageName != "" && buildxMetaData.ImageSha256 != "" {
		return buildxMetaData.ImageName, buildxMetaData.ImageSha256, nil
	}
	// Try read Kaniko/oc file.
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

type buildxMetaData struct {
	ImageName   string `json:"image.name"`
	ImageSha256 string `json:"containerimage.digest"`
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

// Returns a map of: layer-digest -> layer-search-result
func performSearch(imagePathPattern string, serviceManager artifactory.ArtifactoryServicesManager) (resultMap map[string]*utils.ResultItem, err error) {
	searchParams := services.NewSearchParams()
	searchParams.CommonParams = &utils.CommonParams{}
	searchParams.Pattern = imagePathPattern
	var reader *content.ContentReader
	reader, err = serviceManager.SearchFiles(searchParams)
	if err != nil {
		return nil, err
	}
	defer func() {
		if deferErr := reader.Close(); err == nil {
			err = deferErr
		}
	}()
	resultMap = make(map[string]*utils.ResultItem)
	for resultItem := new(utils.ResultItem); reader.NextRecord(resultItem) == nil; resultItem = new(utils.ResultItem) {
		resultMap[resultItem.Name] = resultItem
	}
	err = reader.GetError()
	return
}

// Returns a map of: image-sha2 -> image-layers
func performMultiPlatformImageSearch(imagePathPattern string, serviceManager artifactory.ArtifactoryServicesManager) (resultMap map[string][]*utils.ResultItem, err error) {
	searchParams := services.NewSearchParams()
	searchParams.CommonParams = &utils.CommonParams{}
	searchParams.Pattern = imagePathPattern
	searchParams.Recursive = true
	var reader *content.ContentReader
	reader, err = serviceManager.SearchFiles(searchParams)
	if err != nil {
		return nil, err
	}
	defer func() {
		if deferErr := reader.Close(); err == nil {
			err = deferErr
		}
	}()
	pathToSha2 := make(map[string]string)
	pathToImageLayers := make(map[string][]*utils.ResultItem)
	resultMap = make(map[string][]*utils.ResultItem)
	for resultItem := new(utils.ResultItem); reader.NextRecord(resultItem) == nil; resultItem = new(utils.ResultItem) {
		pathToImageLayers[resultItem.Path] = append(pathToImageLayers[resultItem.Path], resultItem)
		if resultItem.Name == "manifest.json" {
			pathToSha2[resultItem.Path] = "sha256:" + resultItem.Sha256
		}
	}
	for k, v := range pathToSha2 {
		resultMap[v] = append(resultMap[v], pathToImageLayers[k]...)
	}
	err = reader.GetError()
	return
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

func toNoneMarkerLayer(layer string) string {
	imageId := strings.Replace(layer, "__", ":", 1)
	return strings.Replace(imageId, ".marker", "", 1)
}

type CommandType string

// Create a image's build info from manifest.json.
func (builder *buildInfoBuilder) createBuildInfo(commandType CommandType, manifest *manifest, candidateLayers map[string]*utils.ResultItem, module string) (*buildinfo.BuildInfo, error) {
	imageProperties := map[string]string{
		"docker.image.id":  builder.imageSha2,
		"docker.image.tag": builder.image.Name(),
	}
	if module == "" {
		imageName, err := builder.image.GetImageBaseNameWithTag()
		if err != nil {
			return nil, err
		}
		module = imageName
	}
	// Manifest may hold 'empty layers'. As a result, promotion will fail to promote the same layer more than once.
	manifest.Layers = removeDuplicateLayers(manifest.Layers)
	var artifacts []buildinfo.Artifact
	var dependencies []buildinfo.Dependency
	var err error
	switch commandType {
	case Pull:
		dependencies, err = builder.createPullBuildProperties(manifest, candidateLayers)
	case Push:
		artifacts, dependencies, builder.imageLayers, err = builder.createPushBuildProperties(manifest, candidateLayers)
		if err != nil {
			return nil, err
		}
		if !builder.skipTaggingLayers {
			if err := setBuildProperties(builder.buildName, builder.buildNumber, builder.project, builder.imageLayers, builder.serviceManager); err != nil {
				return nil, err
			}
		}
	}
	buildInfo := &buildinfo.BuildInfo{Modules: []buildinfo.Module{{
		Id:           module,
		Type:         buildinfo.Docker,
		Properties:   imageProperties,
		Artifacts:    artifacts,
		Dependencies: dependencies,
	}}}
	return buildInfo, nil
}

// Create the image's build info from list.manifest.json.
func (builder *buildInfoBuilder) createMultiPlatformBuildInfo(fatManifest *FatManifest, searchRultFatManifest *utils.ResultItem, candidateimages map[string][]*utils.ResultItem, module string) (*buildinfo.BuildInfo, error) {
	imageProperties := map[string]string{
		"docker.image.tag": builder.image.Name(),
	}
	if module == "" {
		imageName, err := builder.image.GetImageBaseNameWithTag()
		if err != nil {
			return nil, err
		}
		module = imageName
	}
	// Add layers.
	builder.imageLayers = append(builder.imageLayers, *searchRultFatManifest)
	// Create fat-manifest module
	buildInfo := &buildinfo.BuildInfo{Modules: []buildinfo.Module{{
		Id:         module,
		Type:       buildinfo.Docker,
		Properties: imageProperties,
		Artifacts:  []buildinfo.Artifact{getFatManifestArtifact(searchRultFatManifest)},
	}}}
	// Create all image arch modules
	for _, manifest := range fatManifest.Manifests {
		image := candidateimages[manifest.Digest]
		var artifacts []buildinfo.Artifact
		for _, layer := range image {
			builder.imageLayers = append(builder.imageLayers, *layer)
			if layer.Name == "manifest.json" {
				artifacts = append(artifacts, getManifestArtifact(layer))
			} else {
				artifacts = append(artifacts, layer.ToArtifact())
			}
		}
		buildInfo.Modules = append(buildInfo.Modules, buildinfo.Module{
			Id:        manifest.Platform.Os + "/" + manifest.Platform.Architecture + "/" + module,
			Type:      buildinfo.Docker,
			Artifacts: artifacts,
		})
	}
	return buildInfo, setBuildProperties(builder.buildName, builder.buildNumber, builder.project, builder.imageLayers, builder.serviceManager)
}

func (builder *buildInfoBuilder) createPushBuildProperties(imageManifest *manifest, candidateLayers map[string]*utils.ResultItem) (artifacts []buildinfo.Artifact, dependencies []buildinfo.Dependency, imageLayers []utils.ResultItem, err error) {
	// Add artifacts.
	artifacts = append(artifacts, getManifestArtifact(candidateLayers["manifest.json"]))
	artifacts = append(artifacts, candidateLayers[digestToLayer(builder.imageSha2)].ToArtifact())
	// Add layers.
	imageLayers = append(imageLayers, *candidateLayers["manifest.json"])
	imageLayers = append(imageLayers, *candidateLayers[digestToLayer(builder.imageSha2)])

	totalLayers := len(imageManifest.Layers)
	totalDependencies, err := builder.totalDependencies(candidateLayers[digestToLayer(builder.imageSha2)])
	if err != nil {
		return nil, nil, nil, err
	}
	// Add image layers as artifacts and dependencies.
	for i := 0; i < totalLayers; i++ {
		layerFileName := digestToLayer(imageManifest.Layers[i].Digest)
		item, layerExists := candidateLayers[layerFileName]
		if !layerExists {
			err := handleMissingLayer(imageManifest.Layers[i].MediaType, layerFileName)
			if err != nil {
				return nil, nil, nil, err
			}
			continue
		}
		// Decide if the layer is also a dependency.
		if i < totalDependencies {
			dependencies = append(dependencies, item.ToDependency())
		}
		artifacts = append(artifacts, item.ToArtifact())
		imageLayers = append(imageLayers, *item)
	}
	return
}

func (builder *buildInfoBuilder) createPullBuildProperties(imageManifest *manifest, candidateLayers map[string]*utils.ResultItem) (dependencies []buildinfo.Dependency, err error) {
	// Add dependencies.
	dependencies = append(dependencies, getManifestDependency(candidateLayers["manifest.json"]))
	dependencies = append(dependencies, candidateLayers[digestToLayer(builder.imageSha2)].ToDependency())
	// Add image layers as dependencies.
	for i := 0; i < len(imageManifest.Layers); i++ {
		layerFileName := digestToLayer(imageManifest.Layers[i].Digest)
		item, layerExists := candidateLayers[layerFileName]
		if !layerExists {
			if err := handleMissingLayer(imageManifest.Layers[i].MediaType, layerFileName); err != nil {
				return nil, err
			}
			continue
		}
		dependencies = append(dependencies, item.ToDependency())
	}
	return
}

func (builder *buildInfoBuilder) totalDependencies(image *utils.ResultItem) (int, error) {
	configurationLayer := new(configLayer)
	if err := downloadLayer(*image, &configurationLayer, builder.serviceManager, builder.repositoryDetails.key); err != nil {
		return 0, err
	}
	return configurationLayer.getNumberOfDependentLayers(), nil
}
