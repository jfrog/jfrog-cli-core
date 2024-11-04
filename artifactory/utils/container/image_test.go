package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetImageLongName(t *testing.T) {
	var imageTags = []struct {
		in       string
		expected string
	}{
		{"domain:8080/path:1.0", "path"},
		{"domain:8080/path/in/artifactory:1.0", "path/in/artifactory"},
		{"domain:8080/path/in/artifactory", "path/in/artifactory"},
		{"domain/path:1.0", "path"},
		{"domain/path/in/artifactory:1.0", "path/in/artifactory"},
		{"domain/path/in/artifactory", "path/in/artifactory"},
	}

	for _, v := range imageTags {
		result, err := NewImage(v.in).GetImageLongName()
		assert.NoError(t, err)
		if result != v.expected {
			t.Errorf("GetImageLongName(\"%s\") => '%s', want '%s'", v.in, result, v.expected)
		}
	}
	// Validate failure upon missing image name
	_, err := NewImage("domain").GetImageLongName()
	assert.Error(t, err)

}

func TestGetImageShortName(t *testing.T) {
	var imageTags = []struct {
		in       string
		expected string
	}{
		{"domain:8080/path:1.0", "path"},
		{"domain:8080/path/in/artifactory:1.0", "artifactory"},
		{"domain:8080/path/in/artifactory", "artifactory"},
		{"domain/path:1.0", "path"},
		{"domain/path/in/artifactory:1.0", "artifactory"},
		{"domain/path/in/artifactory", "artifactory"},
	}

	for _, v := range imageTags {
		result, err := NewImage(v.in).GetImageShortName()
		assert.NoError(t, err)
		if result != v.expected {
			t.Errorf("GetImageShortName(\"%s\") => '%s', want '%s'", v.in, result, v.expected)
		}
	}
	// Validate failure upon missing image name
	_, err := NewImage("domain").GetImageShortName()
	assert.Error(t, err)

}

func TestGetImageLongNameWithTag(t *testing.T) {
	var imageTags = []struct {
		in       string
		expected string
	}{
		{"domain:8080/path:1.0", "path:1.0"},
		{"domain:8080/path/in/artifactory:1.0", "path/in/artifactory:1.0"},
		{"domain:8080/path/in/artifactory", "path/in/artifactory:latest"},
		{"domain/path:1.0", "path:1.0"},
		{"domain/path/in/artifactory:1.0", "path/in/artifactory:1.0"},
		{"domain/path/in/artifactory", "path/in/artifactory:latest"},
	}

	for _, v := range imageTags {
		result, err := NewImage(v.in).GetImageLongNameWithTag()
		assert.NoError(t, err)
		if result != v.expected {
			t.Errorf("GetImageLongNameWithTag(\"%s\") => '%s', want '%s'", v.in, result, v.expected)
		}
	}
	// Validate failure upon missing image name
	_, err := NewImage("domain").GetImageLongNameWithTag()
	assert.Error(t, err)
}

func TestGetImageLongNameWithoutRepoWithTag(t *testing.T) {
	var imageTags = []struct {
		in       string
		expected string
	}{
		{"domain:8080/repo-name/hello-world:latest", "hello-world:latest"},
		{"domain/repo-name/hello-world:latest", "hello-world:latest"},
		{"domain/repo-name/org-name/hello-world:latest", "org-name/hello-world:latest"},
		{"domain/repo-name/org-name/hello-world", "org-name/hello-world:latest"},
	}

	for _, v := range imageTags {
		result, err := NewImage(v.in).GetImageLongNameWithoutRepoWithTag()
		assert.NoError(t, err)
		assert.Equal(t, v.expected, result)
	}
	// Validate failure upon missing image name
	_, err := NewImage("domain").GetImageLongNameWithoutRepoWithTag()
	assert.Error(t, err)
}

func TestGetImageShortNameWithTag(t *testing.T) {
	var imageTags = []struct {
		in       string
		expected string
	}{
		{"domain:8080/path:1.0", "path:1.0"},
		{"domain:8080/path/in/artifactory:1.0", "artifactory:1.0"},
		{"domain:8080/path/in/artifactory", "artifactory:latest"},
		{"domain/path:1.0", "path:1.0"},
		{"domain/path/in/artifactory:1.0", "artifactory:1.0"},
		{"domain/path/in/artifactory", "artifactory:latest"},
	}

	for _, v := range imageTags {
		result, err := NewImage(v.in).GetImageShortNameWithTag()
		assert.NoError(t, err)
		if result != v.expected {
			t.Errorf("GetImageShortNameWithTag(\"%s\") => '%s', want '%s'", v.in, result, v.expected)
		}
	}
	// Validate failure upon missing image name
	_, err := NewImage("domain").GetImageLongNameWithTag()
	assert.Error(t, err)
}

func TestResolveRegistryFromTag(t *testing.T) {
	var imageTags = []struct {
		in             string
		expected       string
		expectingError bool
	}{
		{"domain:8080/path:1.0", "domain:8080", false},
		{"domain:8080/path/in/artifactory:1.0", "domain:8080", false},
		{"domain:8080/path/in/artifactory", "domain:8080", false},
		{"domain/path:1.0", "domain", false},
		{"domain/path/in/artifactory:1.0", "domain", false},
		{"domain/path/in/artifactory", "domain", false},
		{"domain:8081", "", true},
	}

	for _, v := range imageTags {
		result, err := NewImage(v.in).GetRegistry()
		if err != nil && !v.expectingError {
			t.Error(err.Error())
		}
		if result != v.expected {
			t.Errorf("ResolveRegistryFromTag(\"%s\") => '%s', expected '%s'", v.in, result, v.expected)
		}
	}
}

func TestDockerClientApiVersionRegex(t *testing.T) {
	var versionStrings = []struct {
		in       string
		expected bool
	}{
		{"1", false},
		{"1.1", true},
		{"1.11", true},
		{"12.12", true},
		{"1.1.11", false},
		{"1.illegal", false},
		{"1 11", false},
	}

	for _, v := range versionStrings {
		result := ApiVersionRegex.Match([]byte(v.in))
		if result != v.expected {
			t.Errorf("Version(\"%s\") => '%v', want '%v'", v.in, result, v.expected)
		}
	}
}

func TestBuildRemoteRepoUrl(t *testing.T) {
	var data = []struct {
		image        string
		isSecure     bool
		expectedRepo string
	}{
		{"localhost:8082/docker-local/hello-world:123", true, "https://localhost:8082/v2/docker-local/hello-world/manifests/123"},
		{"localhost:8082/docker-local/hello-world:latest", true, "https://localhost:8082/v2/docker-local/hello-world/manifests/latest"},
		{"localhost:8082/docker-local/hello-world:latest", false, "http://localhost:8082/v2/docker-local/hello-world/manifests/latest"},
		// With proxy
		{"jfrog-docker-local.jfrog.io/hello-world:123", true, "https://jfrog-docker-local.jfrog.io/v2/hello-world/manifests/123"},
		{"jfrog-docker-local.jfrog.io/hello-world:latest", true, "https://jfrog-docker-local.jfrog.io/v2/hello-world/manifests/latest"},
		{"jfrog-docker-local.jfrog.io/hello-world:123", false, "http://jfrog-docker-local.jfrog.io/v2/hello-world/manifests/123"},
	}
	for _, v := range data {
		testImae := NewImage(v.image)
		containerRegistryUrl, err := testImae.GetRegistry()
		assert.NoError(t, err)

		longImageName, err := testImae.GetImageLongName()
		assert.NoError(t, err)

		imageTag, err := testImae.GetImageTag()
		assert.NoError(t, err)

		actualRepo := buildRequestUrl(longImageName, imageTag, containerRegistryUrl, v.isSecure)
		assert.Equal(t, v.expectedRepo, actualRepo)
	}
}
