package java

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	testsutils "github.com/jfrog/jfrog-cli-core/v2/utils/config/tests"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"

	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"

	"github.com/stretchr/testify/assert"
)

const expectedInitScriptWithRepos = `initscript {
	repositories { 
		mavenCentral()
	}
	dependencies {
		classpath files('%s')
	}
}

allprojects {
	repositories { 
		maven {
			url "https://myartifactory.com/artifactory/deps-repo"
			credentials {
				username = ''
				password = 'my-access-token'
			}
		}
	}
	apply plugin: com.jfrog.GradleDepTree
}`

func TestGradleTreesWithoutConfig(t *testing.T) {
	// Create and change directory to test workspace
	tempDirPath, cleanUp := audit.CreateTestWorkspace(t, "gradle-example-ci-server")
	defer cleanUp()
	assert.NoError(t, os.Chmod(filepath.Join(tempDirPath, "gradlew"), 0700))

	// Run getModulesDependencyTrees
	modulesDependencyTrees, err := buildGradleDependencyTree(&DependencyTreeParams{})
	if assert.NoError(t, err) && assert.NotNil(t, modulesDependencyTrees) {
		assert.Len(t, modulesDependencyTrees, 5)
		// Check module
		module := audit.GetAndAssertNode(t, modulesDependencyTrees, "webservice")
		assert.Len(t, module.Nodes, 7)

		// Check direct dependency
		directDependency := audit.GetAndAssertNode(t, module.Nodes, "junit:junit:4.11")
		assert.Len(t, directDependency.Nodes, 1)

		// Check transitive dependency
		audit.GetAndAssertNode(t, directDependency.Nodes, "org.hamcrest:hamcrest-core:1.3")
	}
}

func TestGradleTreesWithConfig(t *testing.T) {
	// Create and change directory to test workspace
	tempDirPath, cleanUp := audit.CreateTestWorkspace(t, "gradle-example-publish")
	defer cleanUp()
	assert.NoError(t, os.Chmod(filepath.Join(tempDirPath, "gradlew"), 0700))

	// Run getModulesDependencyTrees
	modulesDependencyTrees, err := buildGradleDependencyTree(&DependencyTreeParams{UseWrapper: true})
	if assert.NoError(t, err) && assert.NotNil(t, modulesDependencyTrees) {
		assert.Len(t, modulesDependencyTrees, 5)

		// Check module
		module := audit.GetAndAssertNode(t, modulesDependencyTrees, "api")
		assert.Len(t, module.Nodes, 4)

		// Check direct dependency
		directDependency := audit.GetAndAssertNode(t, module.Nodes, "commons-lang:commons-lang:2.4")
		assert.Len(t, directDependency.Nodes, 1)

		// Check transitive dependency
		audit.GetAndAssertNode(t, directDependency.Nodes, "commons-io:commons-io:1.2")
	}
}

func TestGradleTreesExcludeTestDeps(t *testing.T) {
	// Create and change directory to test workspace
	tempDirPath, cleanUp := audit.CreateTestWorkspace(t, "gradle-example-ci-server")
	defer cleanUp()
	assert.NoError(t, os.Chmod(filepath.Join(tempDirPath, "gradlew"), 0700))

	// Run getModulesDependencyTrees
	modulesDependencyTrees, err := buildGradleDependencyTree(&DependencyTreeParams{UseWrapper: true})
	if assert.NoError(t, err) && assert.NotNil(t, modulesDependencyTrees) {
		assert.Len(t, modulesDependencyTrees, 5)

		// Check direct dependency
		directDependency := audit.GetAndAssertNode(t, modulesDependencyTrees, "services")
		assert.Empty(t, directDependency.Nodes)
	}
}

func TestIsGradleWrapperExist(t *testing.T) {
	// Check Gradle wrapper doesn't exist
	isWrapperExist, err := isGradleWrapperExist()
	assert.False(t, isWrapperExist)
	assert.NoError(t, err)

	// Check Gradle wrapper exist
	_, cleanUp := audit.CreateTestWorkspace(t, "gradle-example-ci-server")
	defer cleanUp()
	isWrapperExist, err = isGradleWrapperExist()
	assert.NoError(t, err)
	assert.True(t, isWrapperExist)
}

func TestGetDepTreeArtifactoryRepository(t *testing.T) {
	tests := []struct {
		name        string
		remoteRepo  string
		server      *config.ServerDetails
		expectedUrl string
		expectedErr string
	}{
		{
			name:       "WithAccessToken",
			remoteRepo: "my-remote-repo",
			server: &config.ServerDetails{
				Url:         "https://myartifactory.com",
				AccessToken: "my-access-token",
			},
			expectedUrl: "\n\t\tmaven {\n\t\t\turl \"/my-remote-repo\"\n\t\t\tcredentials {\n\t\t\t\tusername = ''\n\t\t\t\tpassword = 'my-access-token'\n\t\t\t}\n\t\t}",
			expectedErr: "",
		},
		{
			name:       "WithUsernameAndPassword",
			remoteRepo: "my-remote-repo",
			server: &config.ServerDetails{
				Url:      "https://myartifactory.com",
				User:     "my-username",
				Password: "my-password",
			},
			expectedUrl: "\n\t\tmaven {\n\t\t\turl \"/my-remote-repo\"\n\t\t\tcredentials {\n\t\t\t\tusername = 'my-username'\n\t\t\t\tpassword = 'my-password'\n\t\t\t}\n\t\t}",
			expectedErr: "",
		},
		{
			name:       "MissingCredentials",
			remoteRepo: "my-remote-repo",
			server: &config.ServerDetails{
				Url: "https://myartifactory.com",
			},
			expectedUrl: "",
			expectedErr: "either username/password or access token must be set for https://myartifactory.com",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			url, err := getDepTreeArtifactoryRepository(test.remoteRepo, test.server)
			if err != nil {
				assert.Equal(t, test.expectedErr, err.Error())
			} else {
				assert.Equal(t, test.expectedUrl, url)
			}
		})
	}
}

func TestGetGraphFromDepTree(t *testing.T) {
	// Create and change directory to test workspace
	tempDirPath, cleanUp := audit.CreateTestWorkspace(t, "gradle-example-ci-server")
	defer func() {
		cleanUp()
	}()
	assert.NoError(t, os.Chmod(filepath.Join(tempDirPath, "gradlew"), 0700))
	testCase := struct {
		name           string
		expectedResult map[string]map[string]string
	}{
		name: "ValidOutputFileContent",
		expectedResult: map[string]map[string]string{
			GavPackageTypeIdentifier + "shared":                   {},
			GavPackageTypeIdentifier + filepath.Base(tempDirPath): {},
			GavPackageTypeIdentifier + "services":                 {},
			GavPackageTypeIdentifier + "webservice": {
				GavPackageTypeIdentifier + "junit:junit:4.11":                            "",
				GavPackageTypeIdentifier + "commons-io:commons-io:1.2":                   "",
				GavPackageTypeIdentifier + "org.apache.wicket:wicket:1.3.7":              "",
				GavPackageTypeIdentifier + "org.jfrog.example.gradle:shared:1.0":         "",
				GavPackageTypeIdentifier + "org.jfrog.example.gradle:api:1.0":            "",
				GavPackageTypeIdentifier + "commons-lang:commons-lang:2.4":               "",
				GavPackageTypeIdentifier + "commons-collections:commons-collections:3.2": "",
			},
			GavPackageTypeIdentifier + "api": {
				GavPackageTypeIdentifier + "org.apache.wicket:wicket:1.3.7":      "",
				GavPackageTypeIdentifier + "org.jfrog.example.gradle:shared:1.0": "",
				GavPackageTypeIdentifier + "commons-lang:commons-lang:2.4":       "",
			},
		},
	}

	manager := &depTreeManager{}
	outputFileContent, err := manager.runGradleDepTree()
	assert.NoError(t, err)
	result, err := (&depTreeManager{}).getGraphFromDepTree(outputFileContent)
	assert.NoError(t, err)
	for _, dependency := range result {
		depChild, exists := testCase.expectedResult[dependency.Id]
		assert.True(t, exists)
		assert.Equal(t, len(depChild), len(dependency.Nodes))
	}
}

func TestCreateDepTreeScript(t *testing.T) {
	manager := &depTreeManager{}
	tmpDir, err := manager.createDepTreeScriptAndGetDir()
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, os.Remove(filepath.Join(tmpDir, depTreeInitFile)))
	}()
	content, err := os.ReadFile(filepath.Join(tmpDir, depTreeInitFile))
	assert.NoError(t, err)
	gradleDepTreeJarPath := ioutils.DoubleWinPathSeparator(filepath.Join(tmpDir, gradleDepTreeJarFile))
	assert.Equal(t, fmt.Sprintf(depTreeInitScript, "", gradleDepTreeJarPath, ""), string(content))
}

func TestCreateDepTreeScriptWithRepositories(t *testing.T) {
	manager := &depTreeManager{}
	manager.depsRepo = "deps-repo"
	manager.server = &config.ServerDetails{
		Url:            "https://myartifactory.com/",
		ArtifactoryUrl: "https://myartifactory.com/artifactory",
		AccessToken:    "my-access-token",
	}
	tmpDir, err := manager.createDepTreeScriptAndGetDir()
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, os.Remove(filepath.Join(tmpDir, depTreeInitFile)))
	}()

	content, err := os.ReadFile(filepath.Join(tmpDir, depTreeInitFile))
	assert.NoError(t, err)
	gradleDepTreeJarPath := ioutils.DoubleWinPathSeparator(filepath.Join(tmpDir, gradleDepTreeJarFile))
	assert.Equal(t, fmt.Sprintf(expectedInitScriptWithRepos, gradleDepTreeJarPath), string(content))
}

func TestConstructReleasesRemoteRepo(t *testing.T) {
	cleanUp := testsutils.CreateTempEnv(t, false)
	serverDetails := &config.ServerDetails{
		ServerId:       "test",
		ArtifactoryUrl: "https://domain.com/artifactory",
		User:           "user",
		Password:       "pass",
	}
	err := config.SaveServersConf([]*config.ServerDetails{serverDetails})
	assert.NoError(t, err)
	defer cleanUp()
	testCases := []struct {
		envVar       string
		expectedRepo string
		expectedErr  error
	}{
		{envVar: "", expectedRepo: "", expectedErr: nil},
		{envVar: "test/repo1", expectedRepo: "\n\t\tmaven {\n\t\t\turl \"https://domain.com/artifactory/repo1/artifactory/oss-release-local\"\n\t\t\tcredentials {\n\t\t\t\tusername = 'user'\n\t\t\t\tpassword = 'pass'\n\t\t\t}\n\t\t}", expectedErr: nil},
		{envVar: "notexist/repo1", expectedRepo: "", expectedErr: errors.New("Server ID 'notexist' does not exist.")},
	}

	for _, tc := range testCases {
		// Set the environment variable for this test case
		func() {
			assert.NoError(t, os.Setenv(coreutils.ReleasesRemoteEnv, tc.envVar))
			defer func() {
				// Reset the environment variable after each test case
				assert.NoError(t, os.Unsetenv(coreutils.ReleasesRemoteEnv))
			}()
			actualRepo, actualErr := constructReleasesRemoteRepo()
			assert.Equal(t, tc.expectedRepo, actualRepo)
			assert.Equal(t, tc.expectedErr, actualErr)
		}()
	}
}
