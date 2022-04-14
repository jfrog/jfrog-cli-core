package plugins

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/lock"
	cliLog "github.com/jfrog/jfrog-cli-core/v2/utils/log"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Internal golang locking for the same process.
var mutex sync.Mutex

func init() {
	cliLog.SetDefaultLogger()
}

type PluginsV1 struct {
	Version int `json:"version,omitempty"`
}

func CheckPluginsVersionAndConvertIfNeeded() error {
	// Locking mechanism
	mutex.Lock()
	defer mutex.Unlock()
	lockDirPath, err := coreutils.GetJfrogPluginsLockDir()
	if err != nil {
		return err
	}
	lockFile, err := lock.CreateLock(lockDirPath)
	defer lockFile.Unlock()
	if err != nil {
		return err
	}
	// Check if 'plugins' directory exists in .jfrog
	jfrogHomeDir, err := coreutils.GetJfrogHomeDir()
	if err != nil {
		return err
	}
	exists, err := fileutils.IsDirExists(filepath.Join(jfrogHomeDir, coreutils.JfrogPluginsDirName), false)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	plugins, err := readPluginsYaml()
	if err != nil {
		return err
	}
	if plugins.Version != coreutils.GetPluginsYamlVersion() {
		return errorutils.CheckError(errors.New(fmt.Sprintf("Expected plugins version in 'plugins.yaml is %d  but the actual value is %d", coreutils.GetPluginsYamlVersion(), plugins.Version)))
	}
	return nil
}

func readPluginsYaml() (*PluginsV1, error) {
	plugins := new(PluginsV1)
	content, err := getPluginsYamlFile()
	if err != nil {
		return nil, err
	}
	if len(content) == 0 {
		// No plugins.yaml file was found, that means that we are in v0.
		// Convert plugins layout to the latest version.
		return convertPluginsV0ToV1()
	}

	err = json.Unmarshal(content, &plugins)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	return plugins, err
}

func getPluginsYamlFile() (content []byte, err error) {
	pluginsFilePath, err := getPluginsFilePath()
	if err != nil {
		return
	}
	exists, err := fileutils.IsFileExists(pluginsFilePath, false)
	if err != nil {
		return
	}
	if exists {
		content, err = fileutils.ReadFile(pluginsFilePath)
	}
	return
}

// V0: in plugins directory there was no 'plugins.yaml' file. all executable files were in the same directory.
// V1: create 'plugins.yaml' file inside 'plugins' directory. change the file's hierarchy inside 'plugins' directory.
func convertPluginsV0ToV1() (*PluginsV1, error) {
	err := migrateFileSystemLayoutV0ToV1()
	if err != nil {
		return nil, err
	}
	return CreatePluginsYamlFile()
}

// Change the file's hierarchy inside 'plugins' directory to:
//	plugins (dir)
//		plugin-name (dir)
//			bin (dir)
//				plugin-executable (file)
//			resources:(optional dir)
//				... (directories/files)
func migrateFileSystemLayoutV0ToV1() error {
	plugins, err := coreutils.GetPluginsDirectoryContent()
	if err != nil {
		return err
	}
	pluginsDir, err := coreutils.GetJfrogPluginsDir()
	if err != nil {
		return err
	}
	for _, p := range plugins {
		// Skip 'plugins.yaml'
		if p.Name() == coreutils.JfrogPluginsFile {
			continue
		}
		if p.IsDir() {
			log.Error("unexpected directory in plugins directory")
			break
		}
		// Verify that the file is an executable file
		if !IsExec(p.Mode()) {
			log.Error("unexpected file in plugins directory: " + p.Name())
			continue
		}
		// Move plugins exec files inside a directory with the plugin's name.
		// Create a directory with the plugin's name + "_dir" extension, move the file inside and change directory's name back to plugin's name only.
		pluginsName := getPluginsNameFromExec(p.Name())
		err = os.MkdirAll(filepath.Join(pluginsDir, pluginsName+"_dir", coreutils.PluginsExecDirName), 0777)
		if err != nil {
			return err
		}
		err = fileutils.MoveFile(filepath.Join(pluginsDir, p.Name()), filepath.Join(pluginsDir, pluginsName+"_dir", coreutils.PluginsExecDirName, p.Name()))
		if err != nil {
			return err
		}
		err = os.Chmod(filepath.Join(pluginsDir, pluginsName+"_dir", coreutils.PluginsExecDirName, p.Name()), 0777)
		if err != nil {
			return err
		}
		err = os.Rename(filepath.Join(pluginsDir, pluginsName+"_dir"), filepath.Join(pluginsDir, pluginsName))
		if err != nil {
			return err
		}
	}
	return nil
}

// IsExec Check if file's type is executable
func IsExec(mode os.FileMode) bool {
	return mode&0111 != 0
}

func getPluginsNameFromExec(execName string) string {
	return strings.Split(execName, ".")[0]
}

func CreatePluginsYamlFile() (*PluginsV1, error) {
	pluginsFilePath, err := getPluginsFilePath()
	if err != nil {
		return nil, err
	}
	plugins := new(PluginsV1)
	plugins.Version = 1
	content, err := json.Marshal(plugins)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	err = ioutil.WriteFile(pluginsFilePath, content, 0777)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	return plugins, nil
}

func getPluginsFilePath() (string, error) {
	pluginsFilePath, err := coreutils.GetJfrogHomeDir()
	if err != nil {
		return "", err
	}
	err = os.MkdirAll(pluginsFilePath, 0777)
	if err != nil {
		return "", err
	}
	pluginsFilePath = filepath.Join(pluginsFilePath, coreutils.JfrogPluginsDirName, coreutils.JfrogPluginsFile)
	return pluginsFilePath, nil
}

func GetLocalPluginExecutableName(pluginName string) string {
	if coreutils.IsWindows() {
		return pluginName + ".exe"
	}
	return pluginName
}
