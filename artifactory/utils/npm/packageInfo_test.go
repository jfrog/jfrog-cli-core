package npm

import (
	"reflect"
	"testing"
)

func TestReadPackageInfoFromPackageJson(t *testing.T) {
	tests := []struct {
		json string
		pi   *PackageInfo
	}{
		{`{ "name": "jfrog-cli-tests", "version": "1.0.0", "description": "test package"}`,
			&PackageInfo{Name: "jfrog-cli-tests", Version: "1.0.0", Scope: ""}},
		{`{ "name": "@jfrog/jfrog-cli-tests", "version": "1.0.0", "description": "test package"}`,
			&PackageInfo{Name: "jfrog-cli-tests", Version: "1.0.0", Scope: "@jfrog"}},
	}
	for _, test := range tests {
		t.Run(test.json, func(t *testing.T) {
			packInfo, err := ReadPackageInfo([]byte(test.json))
			if err != nil {
				t.Error("No error was expected in this test", err)
			}

			equals := reflect.DeepEqual(test.pi, packInfo)
			if !equals {
				t.Error("expected:", test.pi, "got:", packInfo)
			}
		})
	}
}

func TestGetDeployPath(t *testing.T) {
	tests := []struct {
		expectedPath string
		pi           *PackageInfo
	}{
		{`jfrog-cli-tests/-/jfrog-cli-tests-1.0.0.tgz`, &PackageInfo{Name: "jfrog-cli-tests", Version: "1.0.0", Scope: ""}},
		{`@jfrog/jfrog-cli-tests/-/jfrog-cli-tests-1.0.0.tgz`, &PackageInfo{Name: "jfrog-cli-tests", Version: "1.0.0", Scope: "@jfrog"}},
	}
	for _, test := range tests {
		t.Run(test.expectedPath, func(t *testing.T) {
			actualPath := test.pi.GetDeployPath()
			if actualPath != test.expectedPath {
				t.Error("expected:", test.expectedPath, "got:", actualPath)
			}
		})
	}
}
