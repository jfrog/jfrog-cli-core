package lifecycle

import (
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/lifecycle/services"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestValidateCreationSources(t *testing.T) {
	testCases := []struct {
		testName                string
		detectedCreationSources []services.SourceType
		errExpected             bool
		errMsg                  string
	}{
		{"missing creation sources", []services.SourceType{}, true, missingCreationSourcesErrMsg},
		{"single creation source", []services.SourceType{services.Aql, services.Artifacts, services.Builds},
			true, multipleCreationSourcesErrMsg + " 'aql, artifacts and builds'"},
		{"single aql err", []services.SourceType{services.Aql, services.Aql}, true, singleAqlErrMsg},
		{"valid aql", []services.SourceType{services.Aql}, false, ""},
		{"valid artifacts", []services.SourceType{services.Artifacts, services.Artifacts}, false, ""},
		{"valid builds", []services.SourceType{services.Builds, services.Builds}, false, ""},
		{"valid release bundles", []services.SourceType{services.ReleaseBundles, services.ReleaseBundles}, false, ""},
	}
	for _, testCase := range testCases {
		t.Run(testCase.testName, func(t *testing.T) {
			err := validateCreationSources(testCase.detectedCreationSources)
			if testCase.errExpected {
				assert.EqualError(t, err, testCase.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateFile(t *testing.T) {
	testCases := []struct {
		testName           string
		file               spec.File
		errExpected        bool
		expectedSourceType services.SourceType
	}{
		{"valid aql", spec.File{Aql: utils.Aql{ItemsFind: "abc"}}, false, services.Aql},
		{"valid build", spec.File{Build: "name/number", IncludeDeps: "true", Project: "project"}, false, services.Builds},
		{"valid bundle", spec.File{Bundle: "name/number", Project: "project"}, false, services.ReleaseBundles},
		{"valid artifacts",
			spec.File{
				Pattern:      "repo/path/file",
				Exclusions:   []string{"exclude"},
				Props:        "prop",
				ExcludeProps: "exclude prop",
				Recursive:    "false"}, false, services.Artifacts},
		{"invalid fields", spec.File{PathMapping: utils.PathMapping{Input: "input"}, Target: "target"}, true, ""},
		{"multiple creation sources",
			spec.File{Aql: utils.Aql{ItemsFind: "abc"}, Build: "name/number", Bundle: "name/number", Pattern: "repo/path/file"},
			true, ""},
		{"invalid aql", spec.File{Aql: utils.Aql{ItemsFind: "abc"}, Props: "prop"}, true, ""},
		{"invalid builds", spec.File{Build: "name/number", Recursive: "false"}, true, ""},
		{"invalid bundles", spec.File{Bundle: "name/number", IncludeDeps: "true"}, true, ""},
		{"invalid artifacts", spec.File{Pattern: "repo/path/file", Project: "proj"}, true, ""},
	}
	for _, testCase := range testCases {
		t.Run(testCase.testName, func(t *testing.T) {
			sourceType, err := validateFile(testCase.file)
			if testCase.errExpected {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expectedSourceType, sourceType)
			}
		})
	}
}
