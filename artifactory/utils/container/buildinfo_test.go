package container

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetManifestPaths(t *testing.T) {
	results := getManifestPaths("/hello-world/latest", "docker-local", Push)
	assert.Len(t, results, 2)
	expected := []string{"docker-local/hello-world/latest/*", "docker-local/latest/*"}
	for i, result := range results {
		assert.Equal(t, expected[i], result)
	}
	results = getManifestPaths("/hello-world/latest", "docker-local", Pull)
	assert.Len(t, results, 4)
	expected = append(expected, "docker-local/library/hello-world/latest/*", "docker-local/library/latest/*")
	for i, result := range results {
		assert.Equal(t, expected[i], result)
	}

	results = getManifestPaths("/docker-remote/hello-world/latest", "docker-remote", Push)
	assert.Len(t, results, 2)
	expected = []string{"docker-remote/docker-remote/hello-world/latest/*", "docker-remote/hello-world/latest/*"}
	for i, result := range results {
		assert.Equal(t, expected[i], result)
	}
	results = getManifestPaths("/docker-remote/hello-world/latest", "docker-remote", Pull)
	assert.Len(t, results, 4)
	expected = append(expected, "docker-remote/library/docker-remote/hello-world/latest/*", "docker-remote/library/hello-world/latest/*")
	for i, result := range results {
		assert.Equal(t, expected[i], result)
	}
}

func TestGetImageWithDigest(t *testing.T) {
	filePath := filepath.Join("..", "testdata", "container", "imageTagWithDigest")
	tag, sha256, err := GetImageTagWithDigest(filePath)
	assert.NoError(t, err)
	assert.Equal(t, "my-image-tag", tag.name)
	assert.Equal(t, "sha256:12345", sha256)
}
