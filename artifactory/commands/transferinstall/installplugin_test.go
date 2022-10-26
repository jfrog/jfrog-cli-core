package transferinstall

import (
	"encoding/json"
	"github.com/jfrog/gofrog/version"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	commonTests "github.com/jfrog/jfrog-cli-core/v2/common/tests"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestFile(t *testing.T) {
	// with dirs
	file := File{"dir", "dir2", "name.txt"}
	dirs := file.Dirs()
	assert.Equal(t, "name.txt", file.Name())
	assert.Len(t, dirs, 2)
	assert.Equal(t, "dir", dirs[0])
	assert.Equal(t, "dir2", dirs[1])
	// no dirs
	file = File{"name2.txt"}
	dirs = file.Dirs()
	assert.Equal(t, "name2.txt", file.Name())
	assert.Len(t, dirs, 0)
	// empty
	file = File{""}
	dirs = file.Dirs()
	assert.Equal(t, "", file.Name())
	assert.Len(t, dirs, 0)
}

func TestToUrl(t *testing.T) {
	file := File{"dir", "dir2", "name.txt"}
	url := toURL(file...)
	assert.Equal(t, "dir/dir2/name.txt", url)
	url = toURL(file.Dirs()...)
	assert.Equal(t, "dir/dir2", url)
	file = File{"name.txt"}
	url = toURL(file...)
	assert.Equal(t, "name.txt", url)
	file = File{""}
	url = toURL(file...)
	assert.Equal(t, "", url)
}

func setUpMockArtifactoryDir(dirs Directory) (rootPath string, clean func(), err error) {
	// prepare temp dir
	rootPath, err = fileutils.CreateTempDir()

	if err != nil {
		return
	}
	rootPath, err = filepath.Abs(rootPath)
	if err != nil {
		return
	}
	// populate a given dir with plugin directory
	err = fileutils.CreateDirIfNotExist(path.Join(rootPath, path.Join(dirs...)))
	if err != nil {
		return
	}
	// set env var to new tmp dir
	oldVal, exists := os.LookupEnv(jHomeEnvVar)
	if err = os.Setenv(jHomeEnvVar, rootPath); err != nil {
		return
	}
	clean = genericCleanFunc(rootPath, exists, oldVal)
	return
}

// creates function to clean up and return to previous state
func genericCleanFunc(rootPath string, envExisted bool, oldEnvVal string) func() {
	return func() {
		// clean temp dir
		homePath, err := filepath.Abs(rootPath)
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}
		errorOccurred := false
		if err = fileutils.RemoveTempDir(homePath); err != nil {
			errorOccurred = true
			log.Error(err)
		}
		// set env to old
		if !envExisted {
			if err = os.Unsetenv(jHomeEnvVar); err != nil {
				errorOccurred = true
				log.Error(err)
			}
		} else {
			if err = os.Setenv(jHomeEnvVar, oldEnvVal); err != nil {
				errorOccurred = true
				log.Error(err)
			}
		}
		if errorOccurred {
			os.Exit(1)
		}
	}
}

func TestValidateRequirements(t *testing.T) {
	// root path env var
	testValidateRootPath(t, true, "") // no env set
	rootPath, clean, err := setUpMockArtifactoryDir(OriginalDirPath)
	assert.NoError(t, err)
	defer clean()
	testValidateRootPath(t, false, rootPath) // env is set
	// min version
	testValidateMinimumVersion(t, "7.0.0", false)               // above
	testValidateMinimumVersion(t, minArtifactoryVersion, false) // exact
	testValidateMinimumVersion(t, "1.0.0", true)                // below
	testValidateMinimumVersion(t, "", true)                     // empty
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

	ver, err := validateAndFetchArtifactoryVersion(serviceManager)
	if errorExpected {
		assert.EqualError(t, err, minVerErr.Error())
		return
	}
	assert.NoError(t, err)
	assert.Equal(t, version.NewVersion(curVersion), ver)
}

func testValidateRootPath(t *testing.T, errorExpected bool, pathExpected string) {
	rootPath, err := validateAndFetchRootPath()
	if errorExpected {
		assert.EqualError(t, err, envVarNotExists.Error())
		return
	}
	assert.Equal(t, pathExpected, rootPath)
	assert.NoError(t, err)
}

func TestTrySearchDestinationMatchFrom(t *testing.T) {
	testTrySearchDestinationMatchFrom(t, OriginalDirPath)
	testTrySearchDestinationMatchFrom(t, V7DirPath)
}

func testTrySearchDestinationMatchFrom(t *testing.T, directory Directory) {
	rootDir, clean, err := setUpMockArtifactoryDir(directory)
	assert.NoError(t, err)
	defer clean()

	manager := NewFileTransferManager(nil)
	// No dest at all
	target, err := manager.trySearchDestinationMatchFrom(rootDir)
	assert.EqualError(t, err, EmptyDestinationErr.Error())
	// with dest not match
	manager.addDestination(Directory{"artifactory", "notdir"})
	target, err = manager.trySearchDestinationMatchFrom(rootDir)
	assert.EqualError(t, err, NotValidDestinationErr(rootDir).Error())
	// with right destination
	manager.addDestination(directory)
	target, err = manager.trySearchDestinationMatchFrom(rootDir)
	assert.NoError(t, err)
	assert.Equal(t, Directory(append(strings.Split(rootDir, "[/\\]"), directory...)), target)
}

func TestGetTransferBundle(t *testing.T) {
	baseUrl := "baseurl"
	v1 := "1.0.0"
	cmd := &InstallPluginCommand{}

	// err - no url provided
	src, action, err := cmd.getTransferBundle()
	assert.EqualError(t, err, EmptyUrlErr.Error())
	cmd.SetBaseDownloadUrl(baseUrl)

	// latest
	src, action, err = cmd.getTransferBundle()
	assert.NoError(t, err)
	assert.Equal(t, toURL(baseUrl, latest), src)
	assert.Contains(t, runtime.FuncForPC(reflect.ValueOf(action).Pointer()).Name(), "transferinstall.DownloadFiles")

	// specific version
	cmd.SetInstallVersion(version.NewVersion(v1))
	src, action, err = cmd.getTransferBundle()
	assert.NoError(t, err)
	assert.Equal(t, toURL(baseUrl, v1), src)
	assert.True(t, strings.Contains(runtime.FuncForPC(reflect.ValueOf(action).Pointer()).Name(), "transferinstall.DownloadFiles"))

	// local file system
	cmd.SetLocalPluginFiles(baseUrl)
	src, action, err = cmd.getTransferBundle()
	assert.NoError(t, err)
	assert.Equal(t, baseUrl, src)
	assert.True(t, strings.Contains(runtime.FuncForPC(reflect.ValueOf(action).Pointer()).Name(), "transferinstall.CopyFiles"))
}

func TestInstallCopy(t *testing.T) {
	fileBundle := FileBundle{
		File{"file1"},
		File{"file2"},
	}
	testInstallCopy(t, OriginalDirPath, fileBundle)
	testInstallCopy(t, V7DirPath, fileBundle)
}

func testInstallCopy(t *testing.T, directory Directory, fileBundle FileBundle) {
	rootPath, clean, err := setUpMockArtifactoryDir(directory)
	assert.NoError(t, err)
	defer clean()
	srcPath, cleanUpSrc, err := createTestTempPluginFiles(t, fileBundle)
	assert.NoError(t, err)
	defer cleanUpSrc()
	dst := path.Join(rootPath, path.Join(directory...))
	// Test when no items in plugin dir (i.e. need to create folders)
	assert.NoError(t, CopyFiles(srcPath, dst, fileBundle))
	// make sure all there
	for _, file := range fileBundle {
		assert.FileExists(t, path.Join(dst, path.Join(file...)))
	}
	// Test when items exist in dir already (i.e. need to override items, no error)
	assert.NoError(t, CopyFiles(srcPath, dst, fileBundle))
}

func createTestTempPluginFiles(t *testing.T, fileBundle FileBundle) (targetPath string, cleanUp func(), err error) {
	// prepare temp dir
	targetPath, err = fileutils.CreateTempDir()
	assert.NoError(t, err)
	targetPath, err = filepath.Abs(targetPath)
	assert.NoError(t, err)
	// generate empty file in dir
	for _, file := range fileBundle {
		assert.NoError(t, ioutil.WriteFile(path.Join(targetPath, path.Join(file.Name())), nil, 0644))
		// create file to confuse
		assert.NoError(t, ioutil.WriteFile(path.Join(targetPath, path.Join(file.Dirs()...), "not_"+file.Name()), nil, 0644))
	}
	cleanUp = func() {
		// clean temp dir
		homePath, err := filepath.Abs(targetPath)
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}
		if err = fileutils.RemoveTempDir(homePath); err != nil {
			log.Error(err)
			os.Exit(1)
		}
	}
	return
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

	assert.NoError(t, sendReLoadCommand(serviceManager))
}
