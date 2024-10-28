package container

import (
	"net/http"
	"path"
	"strings"

	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type Image struct {
	// Image name includes the registry domain, image base name and image tag e.g.: my-registry:port/docker-local/hello-world:latest.
	name string
}

func NewImage(imageTag string) *Image {
	return &Image{name: imageTag}
}

// Get image name
func (image *Image) Name() string {
	return image.name
}

// Get image name from tag by removing the prefixed registry hostname.
// e.g.: https://my-registry/docker-local/hello-world:latest. -> docker-local/hello-world:latest
func (image *Image) GetImageLongNameWithTag() (string, error) {
	if err := image.validateTag(); err != nil {
		return "", err
	}
	indexOfLastSlash := strings.Index(image.name, "/")
	indexOfLastColon := strings.LastIndex(image.name, ":")
	if indexOfLastColon < 0 || indexOfLastColon < indexOfLastSlash {
		log.Info("The image '" + image.name + "' does not include tag. Using the 'latest' tag.")
		image.name += ":latest"
	}
	return image.name[indexOfLastSlash+1:], nil
}

// Get image base name by removing the prefixed registry hostname and the tag.
// e.g.: https://my-registry/docker-local/hello-world:latest. -> docker-local/hello-world
func (image *Image) GetImageLongName() (string, error) {
	imageName, err := image.GetImageLongNameWithTag()
	if err != nil {
		return "", err
	}
	tagIndex := strings.Index(imageName, ":")
	return imageName[:tagIndex], nil
}

func (image *Image) validateTag() error {
	if !strings.Contains(image.name, "/") {
		return errorutils.CheckErrorf("The image '%s' is missing '/' which indicates the image name/tag", image.name)
	}
	return nil
}

// Get image base name by removing the prefixed registry hostname and the tag.
// e.g.: https://my-registry/docker-local/hello-world:latest. -> hello-world
func (image *Image) GetImageShortName() (string, error) {
	imageName, err := image.GetImageShortNameWithTag()
	if err != nil {
		return "", err
	}
	tagIndex := strings.LastIndex(imageName, ":")
	if tagIndex != -1 {
		return imageName[:tagIndex], nil
	}
	return imageName, nil
}

// Get image base name by removing the prefixed registry hostname.
// e.g.: https://my-registry/docker-local/hello-world:latest. -> hello-world:latest
func (image *Image) GetImageShortNameWithTag() (string, error) {
	imageName, err := image.GetImageLongNameWithTag()
	if err != nil {
		return "", err
	}
	indexOfSlash := strings.LastIndex(imageName, "/")
	if indexOfSlash != -1 {
		return imageName[indexOfSlash+1:], nil

	}
	return imageName, nil
}

// GetImageLongNameWithoutRepoWithTag removes the registry hostname and repository name, returning the organization and image name with the tag.
// e.g., "docker-local/myorg/hello-world:latest" -> "myorg/hello-world:latest"
// e.g., "docker-local/hello-world:latest" -> "hello-world:latest"
func (image *Image) GetImageLongNameWithoutRepoWithTag() (string, error) {
	longName, err := image.GetImageLongNameWithTag()
	if err != nil {
		return "", err
	}
	parts := strings.Split(longName, "/")
	if len(parts) > 1 {
		return strings.Join(parts[1:], "/"), nil
	}
	return longName, nil
}

// Get image tag name of an image.
// e.g.: https://my-registry/docker-local/hello-world:latest. -> latest
func (image *Image) GetImageTag() (string, error) {
	imageName, err := image.GetImageLongNameWithTag()
	if err != nil {
		return "", err
	}
	tagIndex := strings.Index(imageName, ":")
	if tagIndex == -1 {
		return "", errorutils.CheckErrorf("unexpected image name '%s'. Failed to get image tag.", image.Name())
	}
	return imageName[tagIndex+1:], nil
}

func (image *Image) GetRegistry() (string, error) {
	if err := image.validateTag(); err != nil {
		return "", err
	}
	indexOfLastSlash := strings.Index(image.name, "/")
	if indexOfLastSlash == -1 {
		return "", errorutils.CheckErrorf("unexpected image name '%s'. Failed to get registry.", image.Name())
	}
	return image.name[:indexOfLastSlash], nil
}

// Returns the physical Artifactory repository name of the pulled/pushed image, by reading a response header from Artifactory.
func (image *Image) GetRemoteRepo(serviceManager artifactory.ArtifactoryServicesManager) (string, error) {
	containerRegistryUrl, err := image.GetRegistry()
	if err != nil {
		return "", err
	}
	longImageName, err := image.GetImageLongName()
	if err != nil {
		return "", err
	}
	imageTag, err := image.GetImageTag()
	if err != nil {
		return "", err
	}
	var isSecure bool
	if rtUrl := serviceManager.GetConfig().GetServiceDetails().GetUrl(); strings.HasPrefix(rtUrl, "https") {
		isSecure = true
	}
	// Build the request URL.
	endpoint := buildRequestUrl(longImageName, imageTag, containerRegistryUrl, isSecure)
	artHttpDetails := serviceManager.GetConfig().GetServiceDetails().CreateHttpClientDetails()
	artHttpDetails.Headers["accept"] = "application/vnd.docker.distribution.manifest.v1+prettyjws, application/json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.index.v1+json"
	resp, _, err := serviceManager.Client().SendHead(endpoint, &artHttpDetails)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", errorutils.CheckErrorf("error while getting docker repository name. Artifactory response: " + resp.Status)
	}
	if dockerRepo := resp.Header["X-Artifactory-Docker-Registry"]; len(dockerRepo) != 0 {
		return dockerRepo[0], nil
	}
	return "", errorutils.CheckErrorf("couldn't find 'X-Artifactory-Docker-Registry' header  docker repository in artifactory")
}

// Returns the name of the repository containing the image in Artifactory.
func buildRequestUrl(longImageName, imageTag, containerRegistryUrl string, https bool) string {
	endpoint := path.Join(containerRegistryUrl, "v2", longImageName, "manifests", imageTag)
	if https {
		return "https://" + endpoint
	}
	return "http://" + endpoint
}
