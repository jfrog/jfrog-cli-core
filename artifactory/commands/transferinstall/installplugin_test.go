package transferinstall

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/gofrog/version"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	commonTests "github.com/jfrog/jfrog-cli-core/v2/common/tests"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestPluginFileItemNameAndDirs(t *testing.T) {
	cases := []struct {
		expectedDirs PluginFileItem
		expectedName string
		file         PluginFileItem
	}{
		{PluginFileItem{"dir", "dir2"}, "name.txt", PluginFileItem{"dir", "dir2", "name.txt"}},
		{PluginFileItem{}, "name.txt", PluginFileItem{"name.txt"}},
		{PluginFileItem{}, "", PluginFileItem{}},
		{PluginFileItem{}, "", PluginFileItem{""}},
		{PluginFileItem{}, "", PluginFileItem{"", "", ""}},
		{PluginFileItem{"dir"}, "", PluginFileItem{"", "dir", ""}},
		{PluginFileItem{"dir"}, "name", PluginFileItem{"dir", "", "name"}},
	}

	for _, testCase := range cases {
		testFileItemNameDirs(t, testCase.expectedDirs, testCase.expectedName, testCase.file)
	}
}

func testFileItemNameDirs(t *testing.T, expectedDirs PluginFileItem, expectedName string, file PluginFileItem) {
	assert.Equal(t, expectedName, file.Name())
	assert.Equal(t, &expectedDirs, file.Dirs())
	fileName, fileDirs := file.SplitNameAndDirs()
	assert.Equal(t, expectedName, fileName)
	assert.Equal(t, &expectedDirs, fileDirs)
	assert.Len(t, *fileDirs, len(expectedDirs))
	for i, DirName := range *fileDirs {
		assert.Equal(t, expectedDirs[i], DirName)
	}
}

func TestPluginFileItemToUrlAndToPath(t *testing.T) {
	cases := []struct {
		expectedUrl  string
		expectedPath string
		file         PluginFileItem
	}{
		{"dir/dir2/name.txt", filepath.Join("dir", "dir2", "name.txt"), PluginFileItem{"dir", "dir2", "name.txt"}},
		{"name.txt", filepath.Join("name.txt"), PluginFileItem{"name.txt"}},
		{"", "", PluginFileItem{}},
		{"", "", PluginFileItem{""}},
		{"", "", PluginFileItem{"", ""}},
		{"dir", filepath.Join("dir", ""), PluginFileItem{"", "dir", ""}},
		{"dir/name", filepath.Join("dir", "name"), PluginFileItem{"dir", "", "name"}},
	}

	for _, testCase := range cases {
		testFileItemToUrlAndToPath(t, testCase.expectedUrl, testCase.expectedPath, testCase.file)
	}
}

func testFileItemToUrlAndToPath(t *testing.T, expectedUrl string, expectedPath string, file PluginFileItem) {
	fUrl, err := file.toURL("")
	assert.NoError(t, err)
	assert.Equal(t, expectedUrl, fUrl)
	assert.Equal(t, expectedPath, file.toPath(""))
	prefix := "prefix"
	assert.Equal(t, filepath.Join(prefix, expectedPath), file.toPath(prefix))
	expectedPrefixUrl := prefix
	if expectedUrl != "" {
		expectedPrefixUrl += "/"
	}
	fUrl, err = file.toURL(prefix)
	assert.NoError(t, err)
	assert.Equal(t, expectedPrefixUrl+expectedUrl, fUrl)
}

func TestValidateRequirements(t *testing.T) {
	// min version
	testValidateMinimumVersion(t, "7.0.0", false)               // above
	testValidateMinimumVersion(t, minArtifactoryVersion, false) // exact
	testValidateMinimumVersion(t, "1.0.0", true)                // below
}

func testValidateMinimumVersion(t *testing.T, curVersion string, errorExpected bool) {
	testServer, _, serviceManager := commonTests.CreateRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/system/version" {
			content, err := json.Marshal(utils.VersionResponse{Version: curVersion})
			assert.NoError(t, err)
			_, err = w.Write(content)
			assert.NoError(t, err)
		}
	})
	defer testServer.Close()

	ver, err := validateMinArtifactoryVersion(serviceManager)
	if errorExpected {
		assert.EqualError(t, err, minVerErr.Error())
		return
	}
	assert.NoError(t, err)
	assert.Equal(t, version.NewVersion(curVersion), ver)
}

func populateDirWith(rootDir string, dirs ...PluginFileItem) {
	for _, dir := range dirs {
		coreutils.ExitOnErr(fileutils.CreateDirIfNotExist(dir.toPath(rootDir)))
	}
}

func setJHomeEnvVar(val string) func() {
	oldVal, exists := os.LookupEnv(jfrogHomeEnvVar)
	if exists && val == "" {
		coreutils.ExitOnErr(os.Unsetenv(jfrogHomeEnvVar))
	} else if val != "" {
		coreutils.ExitOnErr(os.Setenv(jfrogHomeEnvVar, val))
	}

	return func() {
		// set env to old
		if !exists {
			coreutils.ExitOnErr(os.Unsetenv(jfrogHomeEnvVar))
		} else {
			coreutils.ExitOnErr(os.Setenv(jfrogHomeEnvVar, oldVal))
		}
	}
}

func TestSearchDestinationPath(t *testing.T) {
	testDit := PluginFileItem{"test_plugin_install_dir", "test"}
	confuse := PluginFileItem{"test_plugin_install_dir", "test2"} // not destination at all
	manager := &PluginTransferManager{}
	temp, clean := tests.CreateTempDirWithCallbackAndAssert(t)
	defer clean()
	populateDirWith(temp, confuse)
	// no destinations
	exists, target, err := manager.trySearchDestinationMatchFrom(temp)
	assert.NoError(t, err)
	assert.False(t, exists, fmt.Sprintf("the match is %s", target))
	// destination not exists
	manager.addDestination(testDit)
	exists, _, err = manager.trySearchDestinationMatchFrom(temp)
	assert.NoError(t, err)
	assert.False(t, exists)
	// destination exists
	populateDirWith(temp, testDit)
	exists, dst, err := manager.trySearchDestinationMatchFrom(temp)
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, testDit.toPath(temp), dst.toPath())
}

func TestGetPluginDirDestination(t *testing.T) {
	// init mock and test env
	testEnvDir := "testEnv"
	testCustomDir := "testCustom"
	targetDir := "plugins_test_target"
	testHomePath, clean := tests.CreateTempDirWithCallbackAndAssert(t)
	defer clean()
	revert := setJHomeEnvVar("") // reset val to not exists
	defer revert()
	populateDirWith(testHomePath, PluginFileItem{testEnvDir, targetDir}, PluginFileItem{testCustomDir, targetDir})
	manager := NewArtifactoryPluginTransferManager(nil)
	manager.addDestination(PluginFileItem{targetDir})
	cmd := &InstallPluginCommand{transferManger: manager}
	defaultExists, err := fileutils.IsDirExists(defaultSearchPath, false)
	coreutils.ExitOnErr(err)

	// make sure contains artifactory structures as destinations
	assert.Contains(t, manager.destinations, originalDirPath)
	assert.Contains(t, manager.destinations, v7DirPath)

	// default
	dst, err := cmd.getPluginDirDestination()
	if defaultExists {
		assert.NoError(t, err)
		assert.True(t, dst.toPath() == originalDirPath.toPath(defaultSearchPath) || (dst.toPath() == v7DirPath.toPath(defaultSearchPath)))
	} else {
		assert.Errorf(t, err, notValidDestinationErr(true).Error())
	}

	// env var override
	coreutils.ExitOnErr(os.Setenv(jfrogHomeEnvVar, filepath.Join(testHomePath, testEnvDir)))
	dst, err = cmd.getPluginDirDestination()
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(testHomePath, testEnvDir, targetDir), dst.toPath())

	// flag override
	cmd.SetJFrogHomePath(filepath.Join(testHomePath, testCustomDir))
	dst, err = cmd.getPluginDirDestination()
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(testHomePath, testCustomDir, targetDir), dst.toPath())
	cmd.SetJFrogHomePath("not_existing_dir")
	_, err = cmd.getPluginDirDestination()
	assert.Errorf(t, err, notValidDestinationErr(false).Error())
}

func TestGetTransferSourceAndAction(t *testing.T) {
	baseUrl := "baseurl"
	v1 := "1.0.0"
	cmd := &InstallPluginCommand{}

	// err - no url provided with the latest download option
	_, _, err := cmd.getTransferSourceAndAction()
	assert.EqualError(t, err, emptyUrlErr.Error())
	cmd.SetBaseDownloadUrl(baseUrl)

	// latest
	src, action, err := cmd.getTransferSourceAndAction()
	assert.NoError(t, err)
	vUrl, err := url.Parse(baseUrl)
	assert.NoError(t, err)
	assert.Equal(t, path.Join(vUrl.Path, latest) /*toURL(baseUrl, latest)*/, src)
	assert.Contains(t, runtime.FuncForPC(reflect.ValueOf(action).Pointer()).Name(), "transferinstall.DownloadFiles")

	// specific version
	cmd.SetInstallVersion(version.NewVersion(v1))
	src, action, err = cmd.getTransferSourceAndAction()
	assert.NoError(t, err)
	vUrl, err = url.Parse(baseUrl)
	assert.NoError(t, err)
	assert.Equal(t, path.Join(vUrl.Path, v1) /*toURL(baseUrl, v1)*/, src)
	assert.True(t, strings.Contains(runtime.FuncForPC(reflect.ValueOf(action).Pointer()).Name(), "transferinstall.DownloadFiles"))

	// local file system
	cmd.SetLocalPluginFiles(baseUrl)
	src, action, err = cmd.getTransferSourceAndAction()
	assert.NoError(t, err)
	assert.Equal(t, baseUrl, src)
	assert.True(t, strings.Contains(runtime.FuncForPC(reflect.ValueOf(action).Pointer()).Name(), "transferinstall.CopyFiles"))
}

func TestInstallCopy(t *testing.T) {
	fileBundle := PluginFiles{
		PluginFileItem{"file"},
		PluginFileItem{"dir", "file1"},
		PluginFileItem{"dir1", "dir2", "file2"},
	}
	srcPath, cleanUpSrc := tests.CreateTempDirWithCallbackAndAssert(t)
	defer cleanUpSrc()
	tempDst, cleanTempDst := tests.CreateTempDirWithCallbackAndAssert(t)
	defer cleanTempDst()
	dstPath, cleanUpDst := tests.CreateTempDirWithCallbackAndAssert(t)
	defer cleanUpDst()

	// empty
	assert.NoError(t, CopyFiles(srcPath, tempDst, PluginFiles{}))
	// no src files in dir
	assert.Error(t, CopyFiles(srcPath, tempDst, fileBundle))
	// generate empty files in dir (and another to confuse)
	for _, file := range fileBundle {
		assert.NoError(t, ioutil.WriteFile(filepath.Join(srcPath, file.Name()), nil, 0644))
		assert.NoError(t, ioutil.WriteFile(filepath.Join(srcPath, "not_"+file.Name()), nil, 0644))
	}
	// first time in plugin dir (i.e. need to create folders)
	assert.NoError(t, CopyFiles(srcPath, dstPath, fileBundle))
	for _, file := range fileBundle {
		assert.FileExists(t, file.toPath(dstPath))
	}
	// dir already has plugin (i.e. need to override items, no error)
	assert.NoError(t, CopyFiles(srcPath, dstPath, fileBundle))
}

func TestReloadPlugins(t *testing.T) {
	testServer, _, serviceManager := commonTests.CreateRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/"+pluginReloadRestApi {
			w.WriteHeader(http.StatusOK)
			assert.Equal(t, http.MethodPost, r.Method)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	defer testServer.Close()

	assert.NoError(t, sendReloadCommand(serviceManager))
}
