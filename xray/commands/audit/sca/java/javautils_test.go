package java

import (
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/sca"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestGetGraphFromDepTree(t *testing.T) {
	// Create and change directory to test workspace
	tempDirPath, cleanUp := sca.CreateTestWorkspace(t, "gradle-example-ci-server")
	defer func() {
		cleanUp()
	}()
	assert.NoError(t, os.Chmod(filepath.Join(tempDirPath, "gradlew"), 0700))
	testCase := struct {
		name               string
		expectedTree       map[string]map[string]string
		expectedUniqueDeps []string
	}{
		name: "ValidOutputFileContent",
		expectedTree: map[string]map[string]string{
			GavPackageTypeIdentifier + "org.jfrog.example.gradle:shared:1.0":                             {},
			GavPackageTypeIdentifier + "org.jfrog.example.gradle:" + filepath.Base(tempDirPath) + ":1.0": {},
			GavPackageTypeIdentifier + "org.jfrog.example.gradle:services:1.0":                           {},
			GavPackageTypeIdentifier + "org.jfrog.example.gradle:webservice:1.0": {
				GavPackageTypeIdentifier + "junit:junit:4.11":                            "",
				GavPackageTypeIdentifier + "commons-io:commons-io:1.2":                   "",
				GavPackageTypeIdentifier + "org.apache.wicket:wicket:1.3.7":              "",
				GavPackageTypeIdentifier + "org.jfrog.example.gradle:shared:1.0":         "",
				GavPackageTypeIdentifier + "org.jfrog.example.gradle:api:1.0":            "",
				GavPackageTypeIdentifier + "commons-lang:commons-lang:2.4":               "",
				GavPackageTypeIdentifier + "commons-collections:commons-collections:3.2": "",
			},
			GavPackageTypeIdentifier + "org.jfrog.example.gradle:api:1.0": {
				GavPackageTypeIdentifier + "org.apache.wicket:wicket:1.3.7":      "",
				GavPackageTypeIdentifier + "org.jfrog.example.gradle:shared:1.0": "",
				GavPackageTypeIdentifier + "commons-lang:commons-lang:2.4":       "",
			},
		},
		expectedUniqueDeps: []string{
			GavPackageTypeIdentifier + "junit:junit:4.11",
			GavPackageTypeIdentifier + "commons-io:commons-io:1.2",
			GavPackageTypeIdentifier + "org.apache.wicket:wicket:1.3.7",
			GavPackageTypeIdentifier + "org.jfrog.example.gradle:shared:1.0",
			GavPackageTypeIdentifier + "org.jfrog.example.gradle:api:1.0",
			GavPackageTypeIdentifier + "commons-collections:commons-collections:3.2",
			GavPackageTypeIdentifier + "commons-lang:commons-lang:2.4",
			GavPackageTypeIdentifier + "org.hamcrest:hamcrest-core:1.3",
			GavPackageTypeIdentifier + "org.slf4j:slf4j-api:1.4.2",
		},
	}

	manager := &gradleDepTreeManager{}
	outputFileContent, err := manager.runGradleDepTree()
	assert.NoError(t, err)
	depTree, uniqueDeps, err := getGraphFromDepTree(outputFileContent)
	assert.NoError(t, err)
	assert.ElementsMatch(t, uniqueDeps, testCase.expectedUniqueDeps, "First is actual, Second is Expected")

	for _, dependency := range depTree {
		depChild, exists := testCase.expectedTree[dependency.Id]
		assert.True(t, exists)
		assert.Equal(t, len(depChild), len(dependency.Nodes))
	}
}
