package container

import (
	"path"
	"strings"
)

type Image struct {
	tag string
}

//Get image tag
func (image *Image) Tag() string {
	return image.tag
}

// Get image relative path in Artifactory.
func (image *Image) Path() string {
	indexOfFirstSlash := strings.Index(image.tag, "/")
	indexOfLastColon := strings.LastIndex(image.tag, ":")
	if indexOfLastColon < 0 || indexOfLastColon < indexOfFirstSlash {
		return path.Join(image.tag[indexOfFirstSlash:], "latest")
	}
	return path.Join(image.tag[indexOfFirstSlash:indexOfLastColon], image.tag[indexOfLastColon+1:])
}

// Get image name.
func (image *Image) Name() string {
	indexOfLastSlash := strings.LastIndex(image.tag, "/")
	indexOfLastColon := strings.LastIndex(image.tag, ":")
	if indexOfLastColon < 0 || indexOfLastColon < indexOfLastSlash {
		return image.tag[indexOfLastSlash+1:] + ":latest"
	}
	return image.tag[indexOfLastSlash+1:]
}
