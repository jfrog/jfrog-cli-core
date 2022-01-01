package container

import (
	"path"
	"strings"

	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
)

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

type FatManifest struct {
	Manifests []ManifestDetails `json:"manifests"`
}

type ManifestDetails struct {
	Digest   string   `json:"digest"`
	Platform Platform `json:"platform"`
}

type Platform struct {
	Architecture string `json:"architecture"`
	Os           string `json:"os"`
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

func getManifest(resultMap map[string]*utils.ResultItem, serviceManager artifactory.ArtifactoryServicesManager, repo string) (imageManifest *manifest, err error) {
	if len(resultMap) == 0 {
		return
	}
	manifestSearchResult, ok := resultMap["manifest.json"]
	if !ok {
		return
	}
	err = downloadLayer(*manifestSearchResult, &imageManifest, serviceManager, repo)
	return
}

func getFatManifest(resultMap map[string]*utils.ResultItem, serviceManager artifactory.ArtifactoryServicesManager, repo string) (imageFatManifest *FatManifest, err error) {
	if len(resultMap) == 0 {
		return
	}
	manifestSearchResult, ok := resultMap["list.manifest.json"]
	if !ok {
		return
	}
	err = downloadLayer(*manifestSearchResult, &imageFatManifest, serviceManager, repo)
	return
}
