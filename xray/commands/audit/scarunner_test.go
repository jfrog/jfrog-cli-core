package audit

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	xrayutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"

	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"github.com/stretchr/testify/assert"
)

func TestGetDirectDependenciesList(t *testing.T) {
	tests := []struct {
		dependenciesTrees []*xrayUtils.GraphNode
		expectedResult    []string
	}{
		{
			dependenciesTrees: nil,
			expectedResult:    []string{},
		},
		{
			dependenciesTrees: []*xrayUtils.GraphNode{
				{Id: "parent_node_id", Nodes: []*xrayUtils.GraphNode{
					{Id: "issueId_1_direct_dependency", Nodes: []*xrayUtils.GraphNode{{Id: "issueId_1_non_direct_dependency"}}},
					{Id: "issueId_2_direct_dependency", Nodes: nil},
				},
				},
			},
			expectedResult: []string{"issueId_1_direct_dependency", "issueId_2_direct_dependency"},
		},
		{
			dependenciesTrees: []*xrayUtils.GraphNode{
				{Id: "parent_node_id", Nodes: []*xrayUtils.GraphNode{
					{Id: "issueId_1_direct_dependency", Nodes: nil},
					{Id: "issueId_2_direct_dependency", Nodes: nil},
				},
				},
			},
			expectedResult: []string{"issueId_1_direct_dependency", "issueId_2_direct_dependency"},
		},
	}

	for _, test := range tests {
		result := getDirectDependenciesFromTree(test.dependenciesTrees)
		assert.ElementsMatch(t, test.expectedResult, result)
	}
}

func createTestDir(t *testing.T) (directory string, cleanUp func()) {
	tmpDir, err := fileutils.CreateTempDir()
	assert.NoError(t, err)

	// Temp dir structure:
	// tempDir
	// ├── dir
	// │   ├── maven
	// │   │   ├── maven-sub
	// │   │   └── maven-sub
	// │   ├── npm
	// │   └── go
	// ├── yarn
	// │   ├── Pip
	// │   └── Pipenv
	// └── Nuget
	//	   ├── Nuget-sub

	dir := createEmptyDir(t, filepath.Join(tmpDir, "dir"))
	// Maven
	maven := createEmptyDir(t, filepath.Join(dir, "maven"))
	createEmptyFile(t, filepath.Join(maven, "pom.xml"))
	mavenSub := createEmptyDir(t, filepath.Join(maven, "maven-sub"))
	createEmptyFile(t, filepath.Join(mavenSub, "pom.xml"))
	mavenSub2 := createEmptyDir(t, filepath.Join(maven, "maven-sub2"))
	createEmptyFile(t, filepath.Join(mavenSub2, "pom.xml"))
	// Npm
	npm := createEmptyDir(t, filepath.Join(dir, "npm"))
	createEmptyFile(t, filepath.Join(npm, "package.json"))
	createEmptyFile(t, filepath.Join(npm, "package-lock.json"))
	// Go
	goDir := createEmptyDir(t, filepath.Join(dir, "go"))
	createEmptyFile(t, filepath.Join(goDir, "go.mod"))
	// Yarn
	yarn := createEmptyDir(t, filepath.Join(tmpDir, "yarn"))
	createEmptyFile(t, filepath.Join(yarn, "package.json"))
	createEmptyFile(t, filepath.Join(yarn, "yarn.lock"))
	// Pip
	pip := createEmptyDir(t, filepath.Join(yarn, "Pip"))
	createEmptyFile(t, filepath.Join(pip, "requirements.txt"))
	// Pipenv
	pipenv := createEmptyDir(t, filepath.Join(yarn, "Pipenv"))
	createEmptyFile(t, filepath.Join(pipenv, "Pipfile"))
	createEmptyFile(t, filepath.Join(pipenv, "Pipfile.lock"))
	// Nuget
	nuget := createEmptyDir(t, filepath.Join(tmpDir, "Nuget"))
	createEmptyFile(t, filepath.Join(nuget, "project.sln"))
	nugetSub := createEmptyDir(t, filepath.Join(nuget, "Nuget-sub"))
	createEmptyFile(t, filepath.Join(nugetSub, "project.csproj"))

	return tmpDir, func() {
		assert.NoError(t, fileutils.RemoveTempDir(tmpDir), "Couldn't removeAll: "+tmpDir)
	}
}

func createEmptyDir(t *testing.T, path string) string {
	assert.NoError(t, fileutils.CreateDirIfNotExist(path))
	return path
}

func createEmptyFile(t *testing.T, path string) {
	file, err := os.Create(path)
	assert.NoError(t, err)
	file.Close()
}

func TestGetScaScansToPreform(t *testing.T) {

	dir, cleanUp := createTestDir(t)

	tests := []struct {
		name     string
		wd       string
		params   func() *AuditParams
		expected []*xrayutils.ScaScanResult
	}{
		{
			name: "Test specific technologies",
			wd:   dir,
			params: func() *AuditParams {
				param := NewAuditParams()
				param.SetTechnologies([]string{"maven", "npm", "go"})
				return param
			},
			expected: []*xrayutils.ScaScanResult{
				{
					Technology:       coreutils.Maven,
					WorkingDirectory: filepath.Join(dir, "dir", "maven"),
					Descriptors: []string{
						filepath.Join(dir, "dir", "maven", "pom.xml"),
						filepath.Join(dir, "dir", "maven", "maven-sub", "pom.xml"),
						filepath.Join(dir, "dir", "maven", "maven-sub2", "pom.xml"),
					},
				},
				{
					Technology:       coreutils.Npm,
					WorkingDirectory: filepath.Join(dir, "dir", "npm"),
					Descriptors:      []string{filepath.Join(dir, "dir", "npm", "package.json")},
				},
				{
					Technology:       coreutils.Go,
					WorkingDirectory: filepath.Join(dir, "dir", "go"),
					Descriptors:      []string{filepath.Join(dir, "dir", "go", "go.mod")},
				},
			},
		},
		{
			name:   "Test all",
			wd:     dir,
			params: NewAuditParams,
			expected: []*xrayutils.ScaScanResult{
				{
					Technology:       coreutils.Maven,
					WorkingDirectory: filepath.Join(dir, "dir", "maven"),
					Descriptors: []string{
						filepath.Join(dir, "dir", "maven", "pom.xml"),
						filepath.Join(dir, "dir", "maven", "maven-sub", "pom.xml"),
						filepath.Join(dir, "dir", "maven", "maven-sub2", "pom.xml"),
					},
				},
				{
					Technology:       coreutils.Npm,
					WorkingDirectory: filepath.Join(dir, "dir", "npm"),
					Descriptors:      []string{filepath.Join(dir, "dir", "npm", "package.json")},
				},
				{
					Technology:       coreutils.Go,
					WorkingDirectory: filepath.Join(dir, "dir", "go"),
					Descriptors:      []string{filepath.Join(dir, "dir", "go", "go.mod")},
				},
				{
					Technology:       coreutils.Yarn,
					WorkingDirectory: filepath.Join(dir, "yarn"),
					Descriptors:      []string{filepath.Join(dir, "yarn", "package.json")},
				},
				{
					Technology:       coreutils.Pip,
					WorkingDirectory: filepath.Join(dir, "yarn", "Pip"),
					Descriptors:      []string{filepath.Join(dir, "yarn", "Pip", "requirements.txt")},
				},
				{
					Technology:       coreutils.Pipenv,
					WorkingDirectory: filepath.Join(dir, "yarn", "Pipenv"),
					Descriptors:      []string{filepath.Join(dir, "yarn", "Pipenv", "Pipfile")},
				},
				{
					Technology:       coreutils.Nuget,
					WorkingDirectory: filepath.Join(dir, "Nuget"),
					Descriptors:      []string{filepath.Join(dir, "Nuget", "project.sln"), filepath.Join(dir, "Nuget", "Nuget-sub", "project.csproj")},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := getScaScansToPreform(test.wd, test.params())
			for i := range result {
				sort.Strings(result[i].Descriptors)
				sort.Strings(test.expected[i].Descriptors)
			}
			assert.ElementsMatch(t, test.expected, result)
		})
	}

	cleanUp()
}
