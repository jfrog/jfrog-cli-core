package container

import (
	"path"
	"strings"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type Image struct {
	// Image name includes the registry domain, image base name and image tag e.g.: https://my-registry/docker-local/hello-world:latest.
	name string
}

// Get image name
func (image *Image) Name() string {
	return image.name
}

// Get image relative path in Artifactory.
func (image *Image) GetPath() (string, error) {
	if err := image.validateTag(); err != nil {
		return "", err
	}
	indexOfFirstSlash := strings.Index(image.name, "/")
	indexOfLastColon := strings.LastIndex(image.name, ":")
	if indexOfLastColon < 0 || indexOfLastColon < indexOfFirstSlash {
		log.Info("The image '" + image.name + "' does not include tag. Using the 'latest' tag.")
		return path.Join(image.name[indexOfFirstSlash:], "latest"), nil
	}
	return path.Join(image.name[indexOfFirstSlash:indexOfLastColon], image.name[indexOfLastColon+1:]), nil
}

// Get image name from tag by removing the prefixed registry hostname.
func (image *Image) GetImageBaseNameWithTag() (string, error) {
	if err := image.validateTag(); err != nil {
		return "", err
	}
	indexOfLastSlash := strings.LastIndex(image.name, "/")
	indexOfLastColon := strings.LastIndex(image.name, ":")
	if indexOfLastColon < 0 || indexOfLastColon < indexOfLastSlash {
		log.Info("The image '" + image.name + "' does not include tag. Using the 'latest' tag.")
		return image.name[indexOfLastSlash+1:] + ":latest", nil
	}
	return image.name[indexOfLastSlash+1:], nil
}

func (image *Image) validateTag() error {
	if !strings.Contains(image.name, "/") {
		return errorutils.CheckErrorf("The image '%s' is missing '/' which indicates the image name/tag", image.name)
	}
	return nil
}

// Get image base name by removing the prefixed registry hostname and the tag.
// e.g.: https://my-registry/docker-local/hello-world:latest. -> hello-world
func (image *Image) GetImageBaseName() (string, error) {
	imageName, err := image.GetImageBaseNameWithTag()
	if err != nil {
		return "", err
	}
	tagIndex := strings.LastIndex(imageName, ":")

	return imageName[:tagIndex], nil
}
