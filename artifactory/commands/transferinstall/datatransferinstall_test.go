package transferinstall

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/jfrog/gofrog/version"
	commonTests "github.com/jfrog/jfrog-cli-core/v2/common/tests"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	testsutils "github.com/jfrog/jfrog-client-go/utils/tests"
	"github.com/stretchr/testify/assert"
)

func TestPluginFileItemNameAndDirs(t *testing.T) {
	cases := []struct {
		expectedDirs FileItem
		expectedName string
		file         FileItem
	}{
		{FileItem{"dir", "dir2"}, "name.txt", FileItem{"dir", "dir2", "name.txt"}},
		{FileItem{}, "name.txt", FileItem{"name.txt"}},
		{FileItem{}, "", FileItem{}},
		{FileItem{}, "", FileItem{""}},
		{FileItem{}, "", FileItem{"", "", ""}},
		{FileItem{"dir"}, "", FileItem{"", "dir", ""}},
		{FileItem{"dir"}, "name", FileItem{"dir", "", "name"}},
	}

	for _, testCase := range cases {
		testFileItemNameDirs(t, testCase.expectedDirs, testCase.expectedName, testCase.file)
	}
}

func testFileItemNameDirs(t *testing.T, expectedDirs FileItem, expectedName string, file FileItem) {
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
		file         FileItem
	}{
		{"dir/dir2/name.txt", filepath.Join("dir", "dir2", "name.txt"), FileItem{"dir", "dir2", "name.txt"}},
		{"name.txt", "name.txt", FileItem{"name.txt"}},
		{"", "", FileItem{}},
		{"", "", FileItem{""}},
		{"", "", FileItem{"", ""}},
		{"dir", filepath.Join("dir", ""), FileItem{"", "dir", ""}},
		{"dir/name", filepath.Join("dir", "name"), FileItem{"dir", "", "name"}},
	}

	for _, testCase := range cases {
		testFileItemToUrlAndToPath(t, testCase.expectedUrl, testCase.expectedPath, testCase.file)
	}
}

func testFileItemToUrlAndToPath(t *testing.T, expectedUrl string, expectedPath string, file FileItem) {
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

func populateDirWith(rootDir string, dirs ...FileItem) (err error) {
	for _, dir := range dirs {
		if err = fileutils.CreateDirIfNotExist(dir.toPath(rootDir)); err != nil {
			return
		}
	}
	return
}

func TestSearchDestinationPath(t *testing.T) {
	testDir := FileItem{artifactory, "test_plugin_install_dir", "test"}
	confuse := FileItem{artifactory, "test_plugin_install_dir", "test2"}
	manager := &PluginInstallManager{}
	temp, clean := tests.CreateTempDirWithCallbackAndAssert(t)
	defer clean()
	assert.NoError(t, populateDirWith(temp, confuse))
	// No destinations
	exists, target, err := manager.findDestination(temp)
	assert.NoError(t, err)
	assert.False(t, exists, fmt.Sprintf("The match is %s", target))
	// Destination not exists
	manager.addDestination(testDir[1:])
	exists, target, err = manager.findDestination(temp)
	assert.NoError(t, err)
	assert.False(t, exists, fmt.Sprintf("The match is %s", target))
	// Destination exists
	assert.NoError(t, populateDirWith(temp, testDir))
	exists, dst, err := manager.findDestination(temp)
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, testDir.toPath(temp), dst.toPath())
}

func TestGetPluginDirDestination(t *testing.T) {
	// Init mock and test env
	testEnvDir := "testEnv"
	testCustomDir := "testCustom"
	targetDir := "plugins_test_target"
	testHomePath, clean := tests.CreateTempDirWithCallbackAndAssert(t)
	defer clean()
	if oldVal, exists := os.LookupEnv(jfrogHomeEnvVar); exists {
		testsutils.UnSetEnvAndAssert(t, jfrogHomeEnvVar)
		defer testsutils.SetEnvAndAssert(t, jfrogHomeEnvVar, oldVal)
	}
	assert.NoError(t, populateDirWith(testHomePath, FileItem{testEnvDir, artifactory + "-confuse", targetDir}, FileItem{testCustomDir, "confuse-" + artifactory, targetDir}))
	manager := NewArtifactoryPluginInstallManager(nil)
	manager.addDestination(FileItem{targetDir})
	cmd := &InstallDataTransferPluginCommand{transferManger: manager}
	defaultExists, err := fileutils.IsDirExists(defaultSearchPath, false)
	assert.NoError(t, err)

	// Make sure contains artifactory structures as destinations
	assert.Contains(t, manager.destinations, originalDirPath)
	assert.Contains(t, manager.destinations, v7DirPath)

	// Default
	dst, err := cmd.getPluginDirDestination()
	if defaultExists {
		assert.NoError(t, err)
		assert.True(t, dst.toPath() == originalDirPath.toPath(defaultSearchPath) || (dst.toPath() == v7DirPath.toPath(defaultSearchPath)))
	} else {
		assert.Errorf(t, err, errNotValidDestination.Error())
	}

	// Env var override
	testsutils.SetEnvAndAssert(t, jfrogHomeEnvVar, filepath.Join(testHomePath, testEnvDir))
	dst, err = cmd.getPluginDirDestination()
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(testHomePath, testEnvDir, artifactory+"-confuse", targetDir), dst.toPath())

	// Flag override
	cmd.SetJFrogHomePath(filepath.Join(testHomePath, testCustomDir))
	dst, err = cmd.getPluginDirDestination()
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(testHomePath, testCustomDir, "confuse-"+artifactory, targetDir), dst.toPath())
	cmd.SetJFrogHomePath("not_existing_dir")
	_, err = cmd.getPluginDirDestination()
	assert.Errorf(t, err, errNotValidDestination.Error())
}

func TestGetTransferSourceAndAction(t *testing.T) {
	v1 := "1.0.2"
	cmd := &InstallDataTransferPluginCommand{}

	// Latest pinned version
	src, action, err := cmd.getTransferSourceAndAction()
	assert.NoError(t, err)
	vUrl, err := url.Parse(dataTransferUrl)
	assert.NoError(t, err)
	vUrl.Path = path.Join(vUrl.Path, latestPluginVersion)
	assert.Equal(t, vUrl.String(), src)
	assert.NotNil(t, action)

	// Specific version
	cmd.SetInstallVersion(version.NewVersion(v1))
	src, action, err = cmd.getTransferSourceAndAction()
	assert.NoError(t, err)
	vUrl, err = url.Parse(dataTransferUrl)
	assert.NoError(t, err)
	vUrl.Path = path.Join(vUrl.Path, v1)
	assert.Equal(t, vUrl.String(), src)
	// The action must verify downloads against the checksums pinned for the requested version
	server := newPluginFilesServer([]byte("tampered groovy"), []byte("tampered jar"))
	defer server.Close()
	pluginDir, cleanPluginDir := tests.CreateTempDirWithCallbackAndAssert(t)
	defer cleanPluginDir()
	assert.ErrorContains(t, action(server.URL, pluginDir, transferPluginFiles), pluginChecksums[v1][groovyFileName])

	// Version without pinned checksums
	cmd.SetInstallVersion(version.NewVersion("9.9.9"))
	_, action, err = cmd.getTransferSourceAndAction()
	assert.ErrorContains(t, err, "'9.9.9'")
	assert.ErrorContains(t, err, "--dir")
	assert.Nil(t, action)

	// Local file system
	cmd.SetLocalPluginFiles(dataTransferUrl)
	src, action, err = cmd.getTransferSourceAndAction()
	assert.NoError(t, err)
	assert.Equal(t, dataTransferUrl, src)
	assert.True(t, strings.Contains(runtime.FuncForPC(reflect.ValueOf(action).Pointer()).Name(), "transferinstall.CopyFiles"))
}

func TestInstallCopy(t *testing.T) {
	fileBundle := PluginFiles{
		FileItem{"file"},
		FileItem{"dir", "file1"},
		FileItem{"dir1", "dir2", "file2"},
	}
	srcPath, cleanUpSrc := tests.CreateTempDirWithCallbackAndAssert(t)
	defer cleanUpSrc()
	tempDst, cleanTempDst := tests.CreateTempDirWithCallbackAndAssert(t)
	defer cleanTempDst()
	dstPath, cleanUpDst := tests.CreateTempDirWithCallbackAndAssert(t)
	defer cleanUpDst()

	// Empty
	assert.NoError(t, CopyFiles(srcPath, tempDst, PluginFiles{}))
	// No src files in dir
	assert.Error(t, CopyFiles(srcPath, tempDst, fileBundle))
	// Generate empty files in dir (and another to confuse)
	for _, file := range fileBundle {
		assert.NoError(t, os.WriteFile(filepath.Join(srcPath, file.Name()), nil, 0644))
		assert.NoError(t, os.WriteFile(filepath.Join(srcPath, "not_"+file.Name()), nil, 0644))
	}
	// First time in plugin dir (i.e. need to create folders)
	assert.NoError(t, CopyFiles(srcPath, dstPath, fileBundle))
	for _, file := range fileBundle {
		assert.FileExists(t, file.toPath(dstPath))
	}
	// Dir already has plugin (i.e. need to override items, no error)
	assert.NoError(t, CopyFiles(srcPath, dstPath, fileBundle))
}

func TestDownloadTransferInstall(t *testing.T) {
	testDstPath, clean := tests.CreateTempDirWithCallbackAndAssert(t)
	defer clean()
	// Download latest and make sure exists at the end
	assert.NoError(t, DownloadFiles(dataTransferUrl+"/"+latestPluginVersion, testDstPath, transferPluginFiles, pluginChecksums[latestPluginVersion]))
	for _, file := range transferPluginFiles {
		assert.FileExists(t, file.toPath(testDstPath))
	}
}

func TestPluginChecksumsCoverAllPluginFiles(t *testing.T) {
	sha256Pattern := regexp.MustCompile("^[0-9a-f]{64}$")
	assert.Contains(t, pluginChecksums, latestPluginVersion)
	for pluginVersion, checksums := range pluginChecksums {
		for _, file := range transferPluginFiles {
			assert.Regexp(t, sha256Pattern, checksums[file.Name()], "version '%s', file '%s'", pluginVersion, file.Name())
		}
	}
}

func TestDownloadFilesVerifiesChecksums(t *testing.T) {
	groovyContent := []byte("groovy plugin")
	jarContent := []byte("jar library")
	validChecksums := map[string]string{
		groovyFileName: sha256Hex(groovyContent),
		jarFileName:    sha256Hex(jarContent),
	}
	server := newPluginFilesServer(groovyContent, jarContent)
	defer server.Close()

	cases := []struct {
		name        string
		checksums   map[string]string
		expectedErr string
	}{
		{name: "matching checksums", checksums: validChecksums},
		{name: "mismatching checksum", checksums: map[string]string{groovyFileName: validChecksums[groovyFileName], jarFileName: sha256Hex([]byte("other"))}, expectedErr: "SHA-256 checksum of the downloaded plugin file '" + jarFileName + "'"},
		{name: "missing checksum", checksums: map[string]string{groovyFileName: validChecksums[groovyFileName]}, expectedErr: "no expected checksum is known for the plugin file '" + jarFileName + "'"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			pluginDir, cleanPluginDir := tests.CreateTempDirWithCallbackAndAssert(t)
			defer cleanPluginDir()
			err := DownloadFiles(server.URL, pluginDir, transferPluginFiles, testCase.checksums)
			if testCase.expectedErr == "" {
				assert.NoError(t, err)
				for _, file := range transferPluginFiles {
					assert.FileExists(t, file.toPath(pluginDir))
				}
				return
			}
			assert.ErrorContains(t, err, testCase.expectedErr)
			entries, readErr := os.ReadDir(pluginDir)
			assert.NoError(t, readErr)
			assert.Empty(t, entries, "no plugin file may be installed when verification fails")
		})
	}
}

// Serves the plugin files at the same relative paths as the plugin's download URL
func newPluginFilesServer(groovyContent, jarContent []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/" + groovyFileName:
			_, _ = w.Write(groovyContent)
		case "/" + libDir + "/" + jarFileName:
			_, _ = w.Write(jarContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func TestReloadPlugins(t *testing.T) {
	testServer, details, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/"+pluginReloadRestApi {
			w.WriteHeader(http.StatusOK)
			assert.Equal(t, http.MethodPost, r.Method)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	defer testServer.Close()
	installCmd := NewInstallDataTransferCommand(details)
	assert.NoError(t, installCmd.sendReloadRequest())
}
