package plugins

import (
	"encoding/json"
	"errors"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/lock"
	cliLog "github.com/jfrog/jfrog-cli-core/v2/utils/log"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Internal golang locking
// In case the 2 threads in the same needs to check plugins.yml file or migrate the fileSystem files to the latest version.
var mutex sync.Mutex

func init() {
	cliLog.SetDefaultLogger()
}

type PluginsV1 struct {
	Version int `json:"version,omitempty"`
}

// CheckPluginsVersionAndConvertIfNeeded In case the latest plugin's layout version isn't match to the local plugins hierarchy at '.jfrog/plugins' -
// Migrate to the latest version.
func CheckPluginsVersionAndConvertIfNeeded() (err error) {
	// Check if 'plugins' directory exists in .jfrog
	jfrogHomeDir, err := coreutils.GetJfrogHomeDir()
	if err != nil {
		return
	}
	exists, err := fileutils.IsDirExists(filepath.Join(jfrogHomeDir, coreutils.JfrogPluginsDirName), false)
	if err != nil || !exists {
		return
	}

	return readPluginsConfigAndConvertV0tToV1IfNeeded()
}

func readPluginsConfigAndConvertV0tToV1IfNeeded() (err error) {
	content, err := getPluginsConfigFileContent()
	// Return without converting in case of an error, or if the plugins.yml file already exists, which indicates a conversion has already been made.
	if err != nil || len(content) != 0 {
		return
	}
	// Locking mechanism - two threads in the same process.
	mutex.Lock()
	defer mutex.Unlock()
	// Locking mechanism - in case two process would read/migrate local files at '.jfrog/plugins'.
	var lockDirPath string
	lockDirPath, err = coreutils.GetJfrogPluginsLockDir()
	if err != nil {
		return
	}
	var unlockFunc func() error
	unlockFunc, err = lock.CreateLock(lockDirPath)
	// Defer the lockFile.Unlock() function before throwing a possible error to avoid deadlock situations.
	defer func() {
		err = errors.Join(err, unlockFunc())
	}()
	if err != nil {
		return
	}
	// The reason behind reading the config again is that it's possible that another thread or process already changed the plugins file,
	// So we read again inside that locked section to indicate that we indeed need to convert the plugins' layout.
	content, err = getPluginsConfigFileContent()
	if err != nil {
		return
	}
	if len(content) == 0 {
		// No plugins.yaml file was found. This means that we are in v0.
		// Convert plugins layout to the latest version.
		_, err = convertPluginsV0ToV1()
	}
	return
}

func getPluginsConfigFileContent() (content []byte, err error) {
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

// V0: in the plugins directory there was no 'plugins.yaml' file. This means that all executable files are in the same directory.
// V1: We should create a 'plugins.yml' file inside the 'plugins' directory, and also modify the files' hierarchy inside 'plugins' directory.
func convertPluginsV0ToV1() (*PluginsV1, error) {
	err := convertFileSystemLayoutV0ToV1()
	if err != nil {
		return nil, err
	}
	return CreatePluginsConfigFile()
}

// Change the file's hierarchy inside 'plugins' directory to:
//
//	plugins (dir)
//		plugin-name (dir)
//			bin (dir)
//				plugin-executable (file)
//			resources:(optional dir)
//				... (directories/files)
func convertFileSystemLayoutV0ToV1() error {
	plugins, err := coreutils.GetPluginsDirContent()
	if err != nil {
		return err
	}
	pluginsDir, err := coreutils.GetJfrogPluginsDir()
	if err != nil {
		return err
	}
	for _, p := range plugins {
		// Skip 'plugins.yaml'
		if p.Name() == coreutils.JfrogPluginsFileName {
			continue
		}
		if p.IsDir() {
			log.Error("unexpected directory in plugins directory: " + p.Name())
			continue
		}

		pluginsName := removeFileExtension(p.Name())
		// For example case of ".DS_Store" files
		if pluginsName == "" {
			continue
		}
		// Move plugins exec files inside a directory, which has the plugin's name.
		// Create a directory with the plugin's name + "_dir" extension, move the file inside and change directory's name back to plugin's name only.
		pluginDirPathWithExtension := filepath.Join(pluginsDir, pluginsName+"_dir")
		err = os.MkdirAll(filepath.Join(pluginDirPathWithExtension, coreutils.PluginsExecDirName), 0777)
		if err != nil {
			return errorutils.CheckError(err)
		}
		err = fileutils.MoveFile(filepath.Join(pluginsDir, p.Name()), filepath.Join(pluginDirPathWithExtension, coreutils.PluginsExecDirName, p.Name()))
		if err != nil {
			return err
		}
		err = fileutils.MoveDir(pluginDirPathWithExtension, filepath.Join(pluginsDir, pluginsName))
		if err != nil {
			return err
		}
		err = os.RemoveAll(pluginDirPathWithExtension)
		if err != nil {
			return errorutils.CheckError(err)
		}
		err = coreutils.ChmodPluginsDirectoryContent()
		if err != nil {
			return err
		}
	}
	return nil
}

func removeFileExtension(fileName string) string {
	return strings.Split(fileName, ".")[0]
}

func CreatePluginsConfigFile() (*PluginsV1, error) {
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
	err = os.WriteFile(pluginsFilePath, content, 0600)
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
	pluginsFilePath = filepath.Join(pluginsFilePath, coreutils.JfrogPluginsDirName, coreutils.JfrogPluginsFileName)
	return pluginsFilePath, nil
}

func GetLocalPluginExecutableName(pluginName string) string {
	if coreutils.IsWindows() {
		return pluginName + ".exe"
	}
	return pluginName
}
