package java

import (
	"github.com/jfrog/jfrog-cli-security/commands/audit/sca"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGetGradleGraphFromDepTree(t *testing.T) {
	// Create and change directory to test workspace
	tempDirPath, cleanUp := sca.CreateTestWorkspace(t, filepath.Join("projects", "package-managers", "gradle", "gradle"))
	defer cleanUp()
	assert.NoError(t, os.Chmod(filepath.Join(tempDirPath, "gradlew"), 0700))
	expectedTree := map[string]map[string]string{
		"org.jfrog.example.gradle:shared:1.0":                             {},
		"org.jfrog.example.gradle:" + filepath.Base(tempDirPath) + ":1.0": {},
		"org.jfrog.example.gradle:services:1.0":                           {},
		"org.jfrog.example.gradle:webservice:1.0": {
			"junit:junit:4.11":                            "",
			"commons-io:commons-io:1.2":                   "",
			"org.apache.wicket:wicket:1.3.7":              "",
			"org.jfrog.example.gradle:shared:1.0":         "",
			"org.jfrog.example.gradle:api:1.0":            "",
			"commons-lang:commons-lang:2.4":               "",
			"commons-collections:commons-collections:3.2": "",
		},
		"org.jfrog.example.gradle:api:1.0": {
			"org.apache.wicket:wicket:1.3.7":      "",
			"org.jfrog.example.gradle:shared:1.0": "",
			"commons-lang:commons-lang:2.4":       "",
		},
	}
	expectedUniqueDeps := []string{
		"junit:junit:4.11",
		"org.jfrog.example.gradle:webservice:1.0",
		"org.jfrog.example.gradle:api:1.0",
		"org.jfrog.example.gradle:" + filepath.Base(tempDirPath) + ":1.0",
		"commons-io:commons-io:1.2",
		"org.apache.wicket:wicket:1.3.7",
		"org.jfrog.example.gradle:shared:1.0",
		"org.jfrog.example.gradle:api:1.0",
		"commons-collections:commons-collections:3.2",
		"commons-lang:commons-lang:2.4",
		"org.hamcrest:hamcrest-core:1.3",
		"org.slf4j:slf4j-api:1.4.2",
	}

	manager := &gradleDepTreeManager{DepTreeManager{}}
	outputFileContent, err := manager.runGradleDepTree()
	assert.NoError(t, err)
	depTree, uniqueDeps, err := getGraphFromDepTree(outputFileContent)
	assert.NoError(t, err)
	reflect.DeepEqual(uniqueDeps, expectedUniqueDeps)

	for _, dependency := range depTree {
		dependencyId := strings.TrimPrefix(dependency.Id, GavPackageTypeIdentifier)
		depChild, exists := expectedTree[dependencyId]
		assert.True(t, exists)
		assert.Equal(t, len(depChild), len(dependency.Nodes))
	}
}
