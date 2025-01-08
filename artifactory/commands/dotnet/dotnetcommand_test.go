package dotnet

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jfrog/build-info-go/build"
	"github.com/jfrog/build-info-go/build/utils/dotnet"
	"github.com/jfrog/gofrog/io"
	buildUtils "github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	testsutils "github.com/jfrog/jfrog-client-go/utils/tests"
	"github.com/stretchr/testify/assert"
)

func TestGetFlagValueExists(t *testing.T) {
	tests := []struct {
		name              string
		currentConfigPath string
		createConfig      bool
		expectErr         bool
		cmdFlags          []string
		expectedCmdFlags  []string
	}{
		{"simple", "file.config", true, false,
			[]string{"-configFile", "file.config"}, []string{"-configFile", "file.config"}},

		{"simple2", "file.config", true, false,
			[]string{"-before", "-configFile", "file.config", "after"}, []string{"-before", "-configFile", "file.config", "after"}},

		{"err", "file.config", false, true,
			[]string{"-before", "-configFile"}, []string{"-before", "-configFile"}},

		{"err2", "file.config", false, true,
			[]string{"-configFile"}, []string{"-configFile"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.createConfig {
				_, err := io.CreateRandFile(test.currentConfigPath, 0)
				if err != nil {
					t.Error(err)
				}
				defer testsutils.RemoveAndAssert(t, test.currentConfigPath)
			}
			_, err := getFlagValueIfExists("-configfile", test.cmdFlags)
			if err != nil && !test.expectErr {
				t.Error(err)
			}
			if err == nil && test.expectErr {
				t.Errorf("Expecting: error, Got: nil")
			}
			if !reflect.DeepEqual(test.cmdFlags, test.expectedCmdFlags) {
				t.Errorf("Expecting: %s, Got: %s", test.expectedCmdFlags, test.cmdFlags)
			}
		})
	}
}

func TestInitNewConfig(t *testing.T) {
	tmpDir, err := fileutils.CreateTempDir()
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, fileutils.RemoveTempDir(tmpDir))
	}()
	repoName := "test-repo"
	server := &config.ServerDetails{
		ArtifactoryUrl: "https://server.com/artifactory",
		User:           "user",
		Password:       "pass",
	}
	configFile, err := InitNewConfig(tmpDir, repoName, server, false, true)
	assert.NoError(t, err)
	f, err := os.Open(configFile.Name())
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, f.Close())
	}()
	buf := make([]byte, 1024)
	n, err := f.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, `<?xml version="1.0" encoding="utf-8"?>
<configuration>
  <packageSources>
    <add key="JFrogCli" value="https://server.com/artifactory/api/nuget/v3/test-repo" protocolVersion="3" allowInsecureConnections="true"/>
  </packageSources>
  <packageSourceCredentials>
    <JFrogCli>
      <add key="Username" value="user" />
      <add key="ClearTextPassword" value="pass" />
    </JFrogCli>
  </packageSourceCredentials>
</configuration>`, string(buf[:n]))
	server.Password = ""
	server.AccessToken = "abc123"
	configFile, err = InitNewConfig(tmpDir, repoName, server, true, true)
	assert.NoError(t, err)
	updatedConfigFile, err := os.Open(configFile.Name())
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, updatedConfigFile.Close())
	}()
	buf = make([]byte, 1024)
	n, err = updatedConfigFile.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, `<?xml version="1.0" encoding="utf-8"?>
<configuration>
  <packageSources>
    <add key="JFrogCli" value="https://server.com/artifactory/api/nuget/test-repo" protocolVersion="2" allowInsecureConnections="true"/>
  </packageSources>
  <packageSourceCredentials>
    <JFrogCli>
      <add key="Username" value="user" />
      <add key="ClearTextPassword" value="abc123" />
    </JFrogCli>
  </packageSourceCredentials>
</configuration>`, string(buf[:n]))
}

func TestGetSourceDetails(t *testing.T) {
	server := &config.ServerDetails{
		ArtifactoryUrl: "https://server.com/artifactory",
		User:           "user",
		Password:       "pass",
	}
	repoName := "repo-name"
	url, user, pass, err := GetSourceDetails(server, repoName, false)
	assert.NoError(t, err)
	assert.Equal(t, "user", user)
	assert.Equal(t, "pass", pass)
	assert.Equal(t, "https://server.com/artifactory/api/nuget/v3/repo-name", url)
	server.Password = ""
	server.AccessToken = "abc123"
	url, user, pass, err = GetSourceDetails(server, repoName, true)
	assert.Equal(t, "user", user)
	assert.Equal(t, "abc123", pass)
	assert.NoError(t, err)
	assert.Equal(t, "https://server.com/artifactory/api/nuget/repo-name", url)
}

func TestPrepareDotnetBuildInfoModule(t *testing.T) {
	t.Run("generated config file", func(t *testing.T) { testPrepareDotnetBuildInfoModule(t, "restore", []string{}, true) })
	t.Run("existing with configfile flag", func(t *testing.T) {
		testPrepareDotnetBuildInfoModule(t, "restore", []string{"--configfile", "/path/to/config/file"}, false)
	})
	t.Run("existing with source flag", func(t *testing.T) {
		testPrepareDotnetBuildInfoModule(t, "restore", []string{"--source", "/path/to/source"}, false)
	})
	t.Run("dotnet test", func(t *testing.T) {
		testPrepareDotnetBuildInfoModule(t, "test", []string{}, false)
	})
}

func testPrepareDotnetBuildInfoModule(t *testing.T, subCommand string, flags []string, expectedGeneratedConfigFile bool) {
	tmpDir, err := fileutils.CreateTempDir()
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, fileutils.RemoveTempDir(tmpDir))
	}()
	module := createNewDotnetModule(t, tmpDir)
	cmd := DotnetCommand{
		toolchainType:            dotnet.DotnetCore,
		subCommand:               subCommand,
		argAndFlags:              flags,
		buildConfiguration:       buildUtils.NewBuildConfiguration("", "", "mod", ""),
		serverDetails:            &config.ServerDetails{ArtifactoryUrl: "https://my-instance.jfrog.io"},
		allowInsecureConnections: true,
	}
	callbackFunc, err := cmd.prepareDotnetBuildInfoModule(module)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, cmd.toolchainType, module.GetToolchainType())
	assert.Equal(t, cmd.subCommand, module.GetSubcommand())
	assert.Equal(t, cmd.buildConfiguration.GetModule(), module.GetName())

	if !expectedGeneratedConfigFile {
		assertConfigFileNotGenerated(t, cmd, module, tmpDir)
		return
	}
	assertConfigFileGenerated(t, module, callbackFunc)
}

func assertConfigFileNotGenerated(t *testing.T, cmd DotnetCommand, module *build.DotnetModule, tmpDir string) {
	assert.Equal(t, cmd.argAndFlags, module.GetArgAndFlags())
	if cmd.subCommand == "test" {
		assert.True(t, cmd.isDotnetTestCommand())
		assert.Contains(t, cmd.argAndFlags, noRestoreFlag)
	}
	// Temp dir should remain empty if config file was not generated.
	contents, err := os.ReadDir(tmpDir)
	assert.NoError(t, err)
	assert.Empty(t, contents)
}

func assertConfigFileGenerated(t *testing.T, module *build.DotnetModule, callbackFunc func() error) {
	// Assert config file was generated and added to the flags passed to the module.
	assert.Len(t, module.GetArgAndFlags(), 2)
	configFilePath, err := getFlagValueIfExists("--configfile", module.GetArgAndFlags())
	assert.NoError(t, err)
	assertFileExists(t, configFilePath, true)
	assert.True(t, strings.HasPrefix(filepath.Base(configFilePath), configFilePattern))

	// Assert config file is removed when calling the callback function.
	assert.NoError(t, callbackFunc())
	assertFileExists(t, configFilePath, false)
}

func assertFileExists(t *testing.T, path string, expected bool) {
	exists, err := fileutils.IsFileExists(path, false)
	assert.NoError(t, err)
	assert.Equal(t, expected, exists)
}

func createNewDotnetModule(t *testing.T, tmpDir string) *build.DotnetModule {
	dotnetBuild := build.NewBuild("", "", time.Now(), "", tmpDir, nil)
	module, err := dotnetBuild.AddDotnetModules("")
	assert.NoError(t, err)
	return module
}

func TestGetConfigPathFromEnvIfProvided(t *testing.T) {
	testCases := []struct {
		name         string
		mockEnv      map[string]string
		cmdType      dotnet.ToolchainType
		expectedPath string
	}{
		{
			name: "DotnetCore with DOTNET_CLI_HOME",
			mockEnv: map[string]string{
				"DOTNET_CLI_HOME": "/custom/dotnet",
			},
			cmdType:      dotnet.DotnetCore,
			expectedPath: "/custom/dotnet/NuGet.Config",
		},
		{
			name: "NuGet with NUGET_CONFIG_FILE",
			mockEnv: map[string]string{
				"NUGET_CONFIG_FILE": "/custom/nuget.config",
			},
			cmdType:      dotnet.Nuget,
			expectedPath: "/custom/nuget.config",
		},
		{
			name:         "No env variable",
			mockEnv:      map[string]string{},
			cmdType:      dotnet.Nuget,
			expectedPath: "",
		},
	}

	// Test the function with different environment variable settings
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("DOTNET_CLI_HOME", testCase.mockEnv["DOTNET_CLI_HOME"])

			// Set other environment variables if needed
			if testCase.mockEnv["NUGET_CONFIG_FILE"] != "" {
				t.Setenv("NUGET_CONFIG_FILE", testCase.mockEnv["NUGET_CONFIG_FILE"])
			}
			result := GetConfigPathFromEnvIfProvided(testCase.cmdType)
			assert.Equal(t, testCase.expectedPath, ioutils.WinToUnixPathSeparator(result))
		})
	}
}

func TestCreateConfigFileIfNeeded(t *testing.T) {
	testCases := []struct {
		name          string
		configPath    string
		fileExists    bool
		expectedError error
	}{
		{
			name:       "File does not exist, create file with default content",
			configPath: "/custom/path/NuGet.Config",
			fileExists: false,
		},
		{
			name:       "File exists, no changes",
			configPath: "/custom/path/NuGet.Config",
			fileExists: true,
		},
	}

	// Setup for testing file existence and creation
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), testCase.configPath)
			if testCase.fileExists {
				assert.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0777))
				assert.NoError(t, os.WriteFile(configPath, []byte{}, 0644))
			}
			err := CreateConfigFileIfNeeded(configPath)
			assert.NoError(t, err)

			if !testCase.fileExists {
				// Read the content of the file
				content, err := os.ReadFile(configPath)
				assert.NoError(t, err)

				// Assert the content is the default config content
				assert.Equal(t, "<configuration></configuration>", string(content))
			}
		})
	}
}

func TestAddConfigFileFlag(t *testing.T) {
	testCases := []struct {
		name          string
		toolchainType dotnet.ToolchainType
		expectedFlags []string
	}{
		{
			name:          "DotnetCore toolchain",
			toolchainType: dotnet.DotnetCore,
			expectedFlags: []string{"--configfile", "/path/to/NuGet.Config"},
		},
		{
			name:          "NuGet toolchain",
			toolchainType: dotnet.Nuget,
			expectedFlags: []string{"-ConfigFile", "/path/to/NuGet.Config"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Create a mock command object
			cmd, err := dotnet.NewToolchainCmd(testCase.toolchainType)
			assert.NoError(t, err)

			// Call the function
			addConfigFileFlag(cmd, "/path/to/NuGet.Config")

			// Assert that the flags are as expected
			assert.Equal(t, testCase.expectedFlags, cmd.CommandFlags)
		})
	}
}
