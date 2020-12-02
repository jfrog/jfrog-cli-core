package golang

import "testing"

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
