package container

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"

	ioutils "github.com/jfrog/gofrog/io"

	buildinfo "github.com/jfrog/build-info-go/entities"

	artutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	Pull                       CommandType = "pull"
	Push                       CommandType = "push"
	foreignLayerMediaType      string      = "application/vnd.docker.image.rootfs.foreign.diff.tar.gzip"
	imageNotFoundErrorMessage  string      = "Could not find docker image in Artifactory, expecting image tag: %s"
	markerLayerSuffix          string      = ".marker"
	attestationManifestRefType string      = "attestation-manifest"
	unknownPlatformPlaceholder string      = "unknown"

	ManifestJsonFile                  = "manifest.json"
	AttestationsModuleIdPrefix string = "attestations"
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

type RepositoryDetails struct {
	key      string
	isRemote bool
	repoType string
}

// Create instance of docker build info builder.
func newBuildInfoBuilder(image *Image, repository, buildName, buildNumber, project string, serviceManager artifactory.ArtifactoryServicesManager) (*buildInfoBuilder, error) {
	var err error
	builder := &buildInfoBuilder{}
	builder.repositoryDetails.key = repository

	// Get repository details in one API call to determine both isRemote and repoType
	repoDetails := &services.RepositoryDetails{}
	err = serviceManager.GetRepository(repository, &repoDetails)
	if err != nil {
		return nil, errorutils.CheckErrorf("failed to get details for repository '" + repository + "'. Error:\n" + err.Error())
	}

	builder.repositoryDetails.isRemote = repoDetails.GetRepoType() == "remote"
	builder.repositoryDetails.repoType = repoDetails.GetRepoType()

	builder.image = image
	builder.buildName = buildName
	builder.buildNumber = buildNumber
	builder.project = project
	builder.serviceManager = serviceManager
	return builder, nil
}

func (builder *buildInfoBuilder) setImageSha2(imageSha2 string) {
	builder.imageSha2 = imageSha2
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
func setBuildProperties(buildName, buildNumber, project string, imageLayers []utils.ResultItem, serviceManager artifactory.ArtifactoryServicesManager, originalRepo string, repoDetails *RepositoryDetails) (err error) {
	if buildName == "" || buildNumber == "" {
		log.Debug("Skipping setting properties - build name and build number are required")
		return nil
	}

	props, err := build.CreateBuildProperties(buildName, buildNumber, project)
	if err != nil {
		return
	}

	if len(props) == 0 {
		log.Debug("Skipping setting properties - no properties created")
		return nil
	}

	filteredLayers, err := filterLayersForVirtualRepository(imageLayers, serviceManager, originalRepo, repoDetails)
	if err != nil {
		log.Debug("Failed to filter layers for virtual repository, proceeding with all layers:", err.Error())
		filteredLayers = imageLayers
	}

	if len(filteredLayers) == 0 {
		log.Debug("No layers to set properties on, skipping property setting")
		return nil
	}

	pathToFile, err := writeLayersToFile(filteredLayers)
	if err != nil {
		return
	}
	reader := content.NewContentReader(pathToFile, content.DefaultKey)
	defer ioutils.Close(reader, &err)
	_, err = serviceManager.SetProps(services.PropsParams{Reader: reader, Props: props})
	return
}

// filterLayersForVirtualRepository filters image layers to only include those from the default deployment repository
// when dealing with virtual repositories. For non-virtual repositories, it returns all layers unchanged.
func filterLayersForVirtualRepository(imageLayers []utils.ResultItem, serviceManager artifactory.ArtifactoryServicesManager, originalRepo string, repoDetails *RepositoryDetails) ([]utils.ResultItem, error) {
	if len(imageLayers) == 0 {
		return imageLayers, nil
	}

	// Optimization: If we already know the repo type and it's not virtual, skip the API call
	if repoDetails != nil && repoDetails.repoType != "" && repoDetails.repoType != "virtual" {
		log.Debug("Repository ", originalRepo, "is not virtual (type:", repoDetails.repoType+"), skipping determining default deployment config")
		return imageLayers, nil
	}

	// For backwards compatibility or when repoDetails is not available, fall back to API call
	if repoDetails == nil || repoDetails.repoType == "" {
		log.Debug("Repository type not cached, making API call to determine repository configuration")
		repoConfig, err := getRepositoryConfiguration(originalRepo, serviceManager)
		if err != nil {
			return imageLayers, errorutils.CheckErrorf("failed to get repository configuration for '%s': %w", originalRepo, err)
		}

		// If it's not a virtual repository, return all layers unchanged
		if repoConfig == nil || repoConfig.Rclass != "virtual" {
			log.Debug("Repository", originalRepo, "is not virtual, proceeding with all layers")
			return imageLayers, nil
		}

		// If it's a virtual repository but has no default deployment repo, return all layers
		if repoConfig.DefaultDeploymentRepo == "" {
			log.Debug("Virtual repository", originalRepo, "has no default deployment repository, proceeding with all layers")
			return imageLayers, nil
		}

		// Filter layers to only include those from the default deployment repository
		var filteredLayers []utils.ResultItem
		for _, layer := range imageLayers {
			if layer.Repo == repoConfig.DefaultDeploymentRepo {
				filteredLayers = append(filteredLayers, layer)
			}
		}

		if len(filteredLayers) == 0 {
			log.Warn(fmt.Sprintf(`No layers found in default deployment repository '%s' for virtual repository '%s'.
This may indicate that image layers exist in other repositories but not in the default deployment repository.
Properties will not be set to maintain consistency with virtual repository configuration.
To fix this, consider pushing the image directly to the virtual repository to ensure it lands in the default deployment repository.`, repoConfig.DefaultDeploymentRepo, originalRepo))
			return []utils.ResultItem{}, nil
		}
		log.Info("Filtered", len(imageLayers), "layers to", len(filteredLayers), "layers from default deployment repository:", repoConfig.DefaultDeploymentRepo)

		return filteredLayers, nil
	}

	log.Info("Determining virtual repository", originalRepo, "config to determine default deployment repository")
	repoConfig, err := getRepositoryConfiguration(originalRepo, serviceManager)
	if err != nil {
		return imageLayers, errorutils.CheckErrorf("failed to get repository configuration for virtual repository '%s': %w", originalRepo, err)
	}

	// If it's a virtual repository but has no default deployment repo, return all layers
	if repoConfig.DefaultDeploymentRepo == "" {
		log.Debug("Virtual repository", originalRepo, "has no default deployment repository, proceeding with all layers")
		return imageLayers, nil
	}

	// Filter layers to only include those from the default deployment repository
	var filteredLayers []utils.ResultItem
	for _, layer := range imageLayers {
		if layer.Repo == repoConfig.DefaultDeploymentRepo {
			filteredLayers = append(filteredLayers, layer)
		}
	}

	if len(filteredLayers) == 0 {
		log.Warn(fmt.Sprintf(`No layers found in default deployment repository '%s' for virtual repository '%s'. 
This may indicate that image layers exist in other repositories but not in the default deployment repository. 	
Properties will not be set to maintain consistency with virtual repository configuration. 
To fix this, consider pushing the image directly to the virtual repository to ensure it lands in the default deployment repository.`, repoConfig.DefaultDeploymentRepo, originalRepo))
		return []utils.ResultItem{}, nil
	}
	log.Info("Filtered", len(imageLayers), "layers to", len(filteredLayers), "layers from default deployment repository:", repoConfig.DefaultDeploymentRepo)

	return filteredLayers, nil
}

// repositoryConfig represents the virtual repository configuration
type repositoryConfig struct {
	Key                   string `json:"key"`
	Rclass                string `json:"rclass"`
	DefaultDeploymentRepo string `json:"defaultDeploymentRepo"`
}

// getRepositoryConfiguration fetches the repository configuration from Artifactory
func getRepositoryConfiguration(repoKey string, serviceManager artifactory.ArtifactoryServicesManager) (*repositoryConfig, error) {
	httpClientDetails := serviceManager.GetConfig().GetServiceDetails().CreateHttpClientDetails()

	baseUrl := serviceManager.GetConfig().GetServiceDetails().GetUrl()
	endpoint := "api/repositories/" + repoKey
	url := baseUrl + endpoint
	resp, body, _, err := serviceManager.Client().SendGet(url, true, &httpClientDetails)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get repository configuration: HTTP %d", resp.StatusCode)
	}
	var config repositoryConfig
	if err := json.Unmarshal(body, &config); err != nil {
		return nil, fmt.Errorf("failed to parse repository configuration: %v", err)
	}

	return &config, nil
}

// Download the content of layer search result.
func downloadLayer(searchResult utils.ResultItem, result interface{}, serviceManager artifactory.ArtifactoryServicesManager, repo string) error {
	// Search results may include artifacts from the remote-cache repository.
	// When artifact is expired, it cannot be downloaded from the remote-cache.
	// To solve this, change back the search results' repository, to its origin remote/virtual.
	searchResult.Repo = repo
	return artutils.RemoteUnmarshal(serviceManager, searchResult.GetItemRelativePath(), result)
}

func writeLayersToFile(layers []utils.ResultItem) (filePath string, err error) {
	writer, err := content.NewContentWriter("results", true, false)
	if err != nil {
		return
	}
	defer ioutils.Close(writer, &err)
	for _, layer := range layers {
		writer.Write(layer)
	}
	filePath = writer.GetFilePath()
	return
}

// Return - manifest artifacts as buildinfo.Artifact struct.
func getManifestArtifact(manifest *utils.ResultItem) (artifact buildinfo.Artifact) {
	return buildinfo.Artifact{
		Name:                   ManifestJsonFile,
		Type:                   "json",
		Checksum:               buildinfo.Checksum{Sha1: manifest.Actual_Sha1, Md5: manifest.Actual_Md5, Sha256: manifest.Sha256},
		Path:                   path.Join(manifest.Path, manifest.Name),
		OriginalDeploymentRepo: manifest.Repo,
	}
}

// Return - fat manifest artifacts as buildinfo.Artifact struct.
func getFatManifestArtifact(fatManifest *utils.ResultItem) (artifact buildinfo.Artifact) {
	return buildinfo.Artifact{
		Name:                   "list.manifest.json",
		Type:                   "json",
		Checksum:               buildinfo.Checksum{Sha1: fatManifest.Actual_Sha1, Md5: fatManifest.Actual_Md5, Sha256: fatManifest.Sha256},
		Path:                   path.Join(fatManifest.Path, fatManifest.Name),
		OriginalDeploymentRepo: fatManifest.Repo,
	}
}

// Return - manifest dependency as buildinfo.Dependency struct.
func getManifestDependency(searchResults *utils.ResultItem) (dependency buildinfo.Dependency) {
	return buildinfo.Dependency{
		Id:       ManifestJsonFile,
		Type:     "json",
		Checksum: buildinfo.Checksum{Sha1: searchResults.Actual_Sha1, Md5: searchResults.Actual_Md5, Sha256: searchResults.Sha256},
	}
}

// Read the file which contains the following format: 'IMAGE-TAG-IN-ARTIFACTORY'@sha256'SHA256-OF-THE-IMAGE-MANIFEST'.
func GetImageTagWithDigest(filePath string) (*Image, string, error) {
	var buildxMetaData buildxMetaData
	data, err := os.ReadFile(filePath)
	if errorutils.CheckError(err) != nil {
		log.Debug("os.ReadFile failed with '%s'\n", err)
		return nil, "", err
	}
	err = json.Unmarshal(data, &buildxMetaData)
	if err != nil {
		log.Debug("failed unmarshalling buildxMetaData file with error: " + err.Error() + ". falling back to Kanico/OC file format...")
	}
	// Try to read buildx metadata file.
	if buildxMetaData.ImageName != "" && buildxMetaData.ImageSha256 != "" {
		return NewImage(buildxMetaData.ImageName), buildxMetaData.ImageSha256, nil
	}
	// Try read Kaniko/oc file.
	splittedData := strings.Split(string(data), `@`)
	if len(splittedData) != 2 {
		return nil, "", errorutils.CheckErrorf(`unexpected file format "` + filePath + `". The file should include one line in the following format: image-tag@sha256`)
	}
	tag, sha256 := splittedData[0], strings.Trim(splittedData[1], "\n")
	if tag == "" || sha256 == "" {
		err = errorutils.CheckErrorf(`missing image-tag/sha256 in file: "` + filePath + `"`)
		if err != nil {
			return nil, "", err
		}
	}
	return NewImage(tag), sha256, nil
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
	defer ioutils.Close(reader, &err)
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
	defer ioutils.Close(reader, &err)
	pathToSha2 := make(map[string]string)
	pathToImageLayers := make(map[string][]*utils.ResultItem)
	resultMap = make(map[string][]*utils.ResultItem)
	for resultItem := new(utils.ResultItem); reader.NextRecord(resultItem) == nil; resultItem = new(utils.ResultItem) {
		pathToImageLayers[resultItem.Path] = append(pathToImageLayers[resultItem.Path], resultItem)
		if resultItem.Name == ManifestJsonFile {
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

// Create an image's build info from manifest.json.
func (builder *buildInfoBuilder) createBuildInfo(commandType CommandType, manifest *manifest, candidateLayers map[string]*utils.ResultItem, module string) (*buildinfo.BuildInfo, error) {
	if manifest == nil {
		return nil, nil
	}
	imageProperties := map[string]string{
		"docker.image.id":  builder.imageSha2,
		"docker.image.tag": builder.image.Name(),
	}
	if module == "" {
		var err error
		if module, err = builder.image.GetImageShortNameWithTag(); err != nil {
			return nil, err
		}
	}
	// Manifest may hold 'empty layers'. As a result, promotion will fail to promote the same layer more than once.
	manifest.Layers = removeDuplicateLayers(manifest.Layers)
	var artifacts []buildinfo.Artifact
	var dependencies []buildinfo.Dependency
	var err error
	switch commandType {
	case Pull:
		dependencies = builder.createPullBuildProperties(manifest, candidateLayers)
	case Push:
		artifacts, dependencies, builder.imageLayers, err = builder.createPushBuildProperties(manifest, candidateLayers)
		if err != nil {
			return nil, err
		}
		if !builder.skipTaggingLayers {
			if err := setBuildProperties(builder.buildName, builder.buildNumber, builder.project, builder.imageLayers, builder.serviceManager, builder.repositoryDetails.key, &builder.repositoryDetails); err != nil {
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
func (builder *buildInfoBuilder) createMultiPlatformBuildInfo(fatManifest *FatManifest, searchResultFatManifest *utils.ResultItem, candidateImages map[string][]*utils.ResultItem, baseModuleId string) (*buildinfo.BuildInfo, error) {
	imageProperties := map[string]string{
		"docker.image.tag": builder.image.Name(),
	}
	if baseModuleId == "" {
		imageName, err := builder.image.GetImageShortNameWithTag()
		if err != nil {
			return nil, err
		}
		baseModuleId = imageName
	}
	// Add layers.
	builder.imageLayers = append(builder.imageLayers, *searchResultFatManifest)
	// Create fat-manifest module
	buildInfo := &buildinfo.BuildInfo{Modules: []buildinfo.Module{{
		Id:         baseModuleId,
		Type:       buildinfo.Docker,
		Properties: imageProperties,
		Artifacts:  []buildinfo.Artifact{getFatManifestArtifact(searchResultFatManifest)},
	}}}
	imageLongNameWithoutRepo, err := builder.image.GetImageLongNameWithoutRepoWithTag()
	if err != nil {
		return nil, err
	}
	// Create all image arch modules
	for _, manifest := range fatManifest.Manifests {
		image := candidateImages[manifest.Digest]
		var artifacts []buildinfo.Artifact
		for _, layer := range image {
			builder.imageLayers = append(builder.imageLayers, *layer)
			if layer.Name == ManifestJsonFile {
				artifacts = append(artifacts, getManifestArtifact(layer))
			} else {
				artifacts = append(artifacts, layer.ToArtifact())
			}
		}
		buildInfo.Modules = append(buildInfo.Modules, buildinfo.Module{
			Id:        getModuleIdByManifest(manifest, baseModuleId),
			Type:      buildinfo.Docker,
			Artifacts: artifacts,
			Parent:    imageLongNameWithoutRepo,
		})
	}
	return buildInfo, setBuildProperties(builder.buildName, builder.buildNumber, builder.project, builder.imageLayers, builder.serviceManager, builder.repositoryDetails.key, &builder.repositoryDetails)
}

// Construct the manifest's module ID by its type (attestation) or its platform.
func getModuleIdByManifest(manifest ManifestDetails, baseModuleId string) string {
	if manifest.Annotations.ReferenceType == attestationManifestRefType {
		return path.Join(AttestationsModuleIdPrefix, baseModuleId)
	}
	if manifest.Platform.Os != unknownPlatformPlaceholder && manifest.Platform.Architecture != unknownPlatformPlaceholder {
		return path.Join(manifest.Platform.Os, manifest.Platform.Architecture, baseModuleId)
	}
	return baseModuleId
}

func (builder *buildInfoBuilder) createPushBuildProperties(imageManifest *manifest, candidateLayers map[string]*utils.ResultItem) (artifacts []buildinfo.Artifact, dependencies []buildinfo.Dependency, imageLayers []utils.ResultItem, err error) {
	// Add artifacts.
	artifacts = append(artifacts, getManifestArtifact(candidateLayers[ManifestJsonFile]))
	artifacts = append(artifacts, candidateLayers[digestToLayer(builder.imageSha2)].ToArtifact())

	// Add layers.
	imageLayers = append(imageLayers, *candidateLayers[ManifestJsonFile])
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
			err := handleForeignLayer(imageManifest.Layers[i].MediaType, layerFileName)
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

func (builder *buildInfoBuilder) createPullBuildProperties(imageManifest *manifest, imageLayers map[string]*utils.ResultItem) []buildinfo.Dependency {
	configDependencies, err := getDependenciesFromManifestConfig(imageLayers, builder.imageSha2)
	if err != nil {
		log.Debug(err.Error())
		return nil
	}

	layerDependencies, err := getDependenciesFromManifestLayer(imageLayers, imageManifest)
	if err != nil {
		log.Debug(err.Error())
		return nil
	}

	return append(configDependencies, layerDependencies...)
}

func getDependenciesFromManifestConfig(candidateLayers map[string]*utils.ResultItem, imageSha2 string) ([]buildinfo.Dependency, error) {
	var dependencies []buildinfo.Dependency
	manifestSearchResults, found := candidateLayers[ManifestJsonFile]
	if !found {
		return nil, errorutils.CheckErrorf("failed to collect build-info. The manifest.json was not found in Artifactory")
	}

	dependencies = append(dependencies, getManifestDependency(manifestSearchResults))
	imageDetails, found := candidateLayers[digestToLayer(imageSha2)]
	if !found {
		return nil, errorutils.CheckErrorf("failed to collect build-info. Image '" + imageSha2 + "' was not found in Artifactory")
	}

	return append(dependencies, imageDetails.ToDependency()), nil
}

func getDependenciesFromManifestLayer(layers map[string]*utils.ResultItem, imageManifest *manifest) ([]buildinfo.Dependency, error) {
	var dependencies []buildinfo.Dependency
	for i := 0; i < len(imageManifest.Layers); i++ {
		layerFileName := digestToLayer(imageManifest.Layers[i].Digest)
		item, layerExists := layers[layerFileName]
		if !layerExists {
			if err := handleForeignLayer(imageManifest.Layers[i].MediaType, layerFileName); err != nil {
				return nil, err
			}
			continue
		}
		dependencies = append(dependencies, item.ToDependency())
	}
	return dependencies, nil
}

func (builder *buildInfoBuilder) totalDependencies(image *utils.ResultItem) (int, error) {
	configurationLayer := new(configLayer)
	if err := downloadLayer(*image, &configurationLayer, builder.serviceManager, builder.repositoryDetails.key); err != nil {
		return 0, err
	}
	return configurationLayer.getNumberOfDependentLayers(), nil
}
