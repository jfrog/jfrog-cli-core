package container

import (
	"path"
	"strings"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type Image struct {
	tag string
}

// Get image tag
func (image *Image) Tag() string {
	return image.tag
}

// Get image relative path in Artifactory.
func (image *Image) Path() (string, error) {
	if err := image.validateTag(); err != nil {
		return "", err
	}
	indexOfFirstSlash := strings.Index(image.tag, "/")
	indexOfLastColon := strings.LastIndex(image.tag, ":")
	if indexOfLastColon < 0 || indexOfLastColon < indexOfFirstSlash {
		log.Info("The image '" + image.tag + "' does not include tag. Using the 'latest' tag.")
		return path.Join(image.tag[indexOfFirstSlash:], "latest"), nil
	}
	return path.Join(image.tag[indexOfFirstSlash:indexOfLastColon], image.tag[indexOfLastColon+1:]), nil
}

// Get image name.
func (image *Image) Name() (string, error) {
	if err := image.validateTag(); err != nil {
		return "", err
	}
	indexOfLastSlash := strings.LastIndex(image.tag, "/")
	indexOfLastColon := strings.LastIndex(image.tag, ":")
	if indexOfLastColon < 0 || indexOfLastColon < indexOfLastSlash {
		log.Info("The image '" + image.tag + "' does not include tag. Using the 'latest' tag.")
		return image.tag[indexOfLastSlash+1:] + ":latest", nil
	}
	return image.tag[indexOfLastSlash+1:], nil
}

func (image *Image) validateTag() error {
	if !strings.Contains(image.tag, "/") {
		return errorutils.CheckErrorf("The image '%s' is missing '/' which indicates the image name/tag", image.tag)
	}
	return nil
}
