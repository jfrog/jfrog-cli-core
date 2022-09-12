package dotnet

import (
	"github.com/jfrog/build-info-go/build"
	"github.com/jfrog/build-info-go/build/utils/dotnet"
	"github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	testsutils "github.com/jfrog/jfrog-client-go/utils/tests"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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

func TestPrepareDotnetBuildInfoModule(t *testing.T) {
	t.Run("generated config file", func(t *testing.T) { testPrepareDotnetBuildInfoModule(t, []string{}, true) })
	t.Run("existing with configfile flag", func(t *testing.T) {
		testPrepareDotnetBuildInfoModule(t, []string{"--configfile", "/path/to/config/file"}, false)
	})
	t.Run("existing with source flag", func(t *testing.T) {
		testPrepareDotnetBuildInfoModule(t, []string{"--source", "/path/to/source"}, false)
	})
}

func testPrepareDotnetBuildInfoModule(t *testing.T, flags []string, expectedGeneratedConfigFile bool) {
	tmpDir, err := fileutils.CreateTempDir()
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, fileutils.RemoveTempDir(tmpDir))
	}()
	module := createNewDotnetModule(t, tmpDir)
	cmd := DotnetCommand{
		toolchainType:      dotnet.DotnetCore,
		subCommand:         "restore",
		argAndFlags:        flags,
		buildConfiguration: utils.NewBuildConfiguration("", "", "mod", ""),
		serverDetails:      &config.ServerDetails{ArtifactoryUrl: "https://my-instance.jfrog.io"},
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
	dotnetBuild := build.NewBuild("", "", "", tmpDir, nil)
	module, err := dotnetBuild.AddDotnetModules("")
	assert.NoError(t, err)
	return module
}
