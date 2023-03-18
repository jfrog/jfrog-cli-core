package golang

import (
	goutils "github.com/jfrog/jfrog-cli-core/v2/utils/golang"
	testsutils "github.com/jfrog/jfrog-client-go/utils/tests"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildPackageVersionRequest(t *testing.T) {
	tests := []struct {
		packageName     string
		branchName      string
		expectedRequest string
	}{
		{"github.com/jfrog/jfrog-cli", "", "github.com/jfrog/jfrog-cli/@v/latest.info"},
		{"github.com/jfrog/jfrog-cli", "dev", "github.com/jfrog/jfrog-cli/@v/dev.info"},
		{"github.com/jfrog/jfrog-cli", "v1.0.7", "github.com/jfrog/jfrog-cli/@v/v1.0.7.info"},
	}
	for _, test := range tests {
		t.Run(test.expectedRequest, func(t *testing.T) {
			versionRequest := buildPackageVersionRequest(test.packageName, test.branchName)
			if versionRequest != test.expectedRequest {
				t.Error("Failed to build package version request. The version request is", versionRequest, " but it is expected to be", test.expectedRequest)
			}
		})
	}
}

func TestGetPackageFilesPath(t *testing.T) {
	packageCachePath, err := goutils.GetGoModCachePath()
	assert.NoError(t, err)
	packageName := "github.com/golang/mock/mockgen"
	version := "v1.4.1"
	expectedPackagePath := filepath.Join(packageCachePath, "github.com/golang/mock@"+version)
	err = os.MkdirAll(expectedPackagePath, os.ModePerm)
	assert.NoError(t, err)
	defer testsutils.RemoveAllAndAssert(t, expectedPackagePath)
	actualPackagePath, err := getFileSystemPackagePath(packageCachePath, packageName, version)
	assert.NoError(t, err)
	assert.Equal(t, expectedPackagePath, actualPackagePath)
}
