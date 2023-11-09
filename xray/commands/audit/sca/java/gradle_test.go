package java

import (
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/sca"
	"os"
	"path/filepath"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	testsutils "github.com/jfrog/jfrog-cli-core/v2/utils/config/tests"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"

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
	tempDirPath, cleanUp := sca.CreateTestWorkspace(t, "gradle-example-ci-server")
	defer cleanUp()
	assert.NoError(t, os.Chmod(filepath.Join(tempDirPath, "gradlew"), 0700))

	// Run getModulesDependencyTrees
	modulesDependencyTrees, uniqueDeps, err := buildGradleDependencyTree(&DepTreeParams{})
	if assert.NoError(t, err) && assert.NotNil(t, modulesDependencyTrees) {
		assert.Len(t, uniqueDeps, 12)
		assert.Len(t, modulesDependencyTrees, 5)
		// Check module
		module := sca.GetAndAssertNode(t, modulesDependencyTrees, "org.jfrog.example.gradle:webservice:1.0")
		assert.Len(t, module.Nodes, 7)

		// Check direct dependency
		directDependency := sca.GetAndAssertNode(t, module.Nodes, "junit:junit:4.11")
		assert.Len(t, directDependency.Nodes, 1)

		// Check transitive dependency
		sca.GetAndAssertNode(t, directDependency.Nodes, "org.hamcrest:hamcrest-core:1.3")
	}
}

func TestGradleTreesWithConfig(t *testing.T) {
	// Create and change directory to test workspace
	tempDirPath, cleanUp := sca.CreateTestWorkspace(t, "gradle-example-publish")
	defer cleanUp()
	assert.NoError(t, os.Chmod(filepath.Join(tempDirPath, "gradlew"), 0700))

	// Run getModulesDependencyTrees
	modulesDependencyTrees, uniqueDeps, err := buildGradleDependencyTree(&DepTreeParams{UseWrapper: true})
	if assert.NoError(t, err) && assert.NotNil(t, modulesDependencyTrees) {
		assert.Len(t, modulesDependencyTrees, 5)
		assert.Len(t, uniqueDeps, 8)
		// Check module
		module := sca.GetAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test.gradle.publish:api:1.0-SNAPSHOT")
		assert.Len(t, module.Nodes, 4)

		// Check direct dependency
		directDependency := sca.GetAndAssertNode(t, module.Nodes, "commons-lang:commons-lang:2.4")
		assert.Len(t, directDependency.Nodes, 1)

		// Check transitive dependency
		sca.GetAndAssertNode(t, directDependency.Nodes, "commons-io:commons-io:1.2")
	}
}

func TestIsGradleWrapperExist(t *testing.T) {
	// Check Gradle wrapper doesn't exist
	isWrapperExist, err := isGradleWrapperExist()
	assert.False(t, isWrapperExist)
	assert.NoError(t, err)

	// Check Gradle wrapper exist
	_, cleanUp := sca.CreateTestWorkspace(t, "gradle-example-ci-server")
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

func TestCreateDepTreeScript(t *testing.T) {
	manager := &gradleDepTreeManager{&DepTreeManager{}}
	tmpDir, err := manager.createDepTreeScriptAndGetDir()
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, os.Remove(filepath.Join(tmpDir, gradleDepTreeInitFile)))
	}()
	content, err := os.ReadFile(filepath.Join(tmpDir, gradleDepTreeInitFile))
	assert.NoError(t, err)
	gradleDepTreeJarPath := ioutils.DoubleWinPathSeparator(filepath.Join(tmpDir, gradleDepTreeJarFile))
	assert.Equal(t, fmt.Sprintf(gradleDepTreeInitScript, "", gradleDepTreeJarPath, ""), string(content))
}

func TestCreateDepTreeScriptWithRepositories(t *testing.T) {
	manager := &gradleDepTreeManager{&DepTreeManager{}}
	manager.depsRepo = "deps-repo"
	manager.server = &config.ServerDetails{
		Url:            "https://myartifactory.com/",
		ArtifactoryUrl: "https://myartifactory.com/artifactory",
		AccessToken:    "my-access-token",
	}
	tmpDir, err := manager.createDepTreeScriptAndGetDir()
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, os.Remove(filepath.Join(tmpDir, gradleDepTreeInitFile)))
	}()

	content, err := os.ReadFile(filepath.Join(tmpDir, gradleDepTreeInitFile))
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
