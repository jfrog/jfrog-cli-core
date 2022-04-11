package plugins

import (
	"bytes"
	"encoding/json"
	"github.com/buger/jsonparser"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	cliLog "github.com/jfrog/jfrog-cli-core/v2/utils/log"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func init() {
	cliLog.SetDefaultLogger()
}

type PluginsV1 struct {
	Version int `json:"version,omitempty"`
}

func CheckPluginsVersionAndConvertIfNeeded() error {
	plugins, err := readPluginsYaml()
	if err != nil {
		return err
	}
	if plugins.Version != coreutils.GetPluginsVersion() {
		return errorutils.CheckError(errors.New("Expected plugins version in 'plugins.yaml is " + string(coreutils.GetPluginsVersion()) + " but the actual value is" + string(plugins.Version)))
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
		// No plugins.yaml file was found, that meant that we are in V0.
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

// Creating a homedir backup prior to converting.
func createHomeDirBackup() error {
	homeDir, err := coreutils.GetJfrogHomeDir()
	if err != nil {
		return err
	}
	backupDir, err := coreutils.GetJfrogBackupDir()
	if err != nil {
		return err
	}

	// Copy homedir contents to backup dir, excluding redundant dirs and the backup dir itself.
	backupName := ".jfrog-" + strconv.FormatInt(time.Now().Unix(), 10)
	curBackupPath := filepath.Join(backupDir, backupName)
	log.Debug("Creating a homedir backup at: " + curBackupPath)
	exclude := []string{coreutils.JfrogBackupDirName, coreutils.JfrogDependenciesDirName, coreutils.JfrogLocksDirName, coreutils.JfrogLogsDirName}
	return fileutils.CopyDir(homeDir, curBackupPath, true, exclude)
}

// Version key doesn't exist in version 0
// Version key is "Version" in version 1
// Version key is "version" in version 2 and above
func getVersion(content []byte) (value string, err error) {
	value, err = jsonparser.GetString(bytes.ToLower(content), "version")
	if err != nil && err.Error() == "Key path not found" {
		return "0", nil
	}
	return value, errorutils.CheckError(err)
}

// V0: in plugins directory there was no 'plugins.yaml' file. all executable files were in the same directory.
// V1: create 'plugins.yaml' file inside 'plugins' directory. change the file's hierarchy inside 'plugins' directory.
func convertPluginsV0ToV1() (*PluginsV1, error) {
	// TODO: what to do after migration fails?
	err := createHomeDirBackup()
	if err != nil {
		return nil, err
	}
	err = migrateFileSystemLayout()
	if err != nil {
		return nil, err
	}
	return createPluginsYamlFile()
}

//change the file's hierarchy inside 'plugins' directory to:
//	plugins:
//		<plugin-name>:
//			bin:
//				<plugin-exec>
//			resources:(optional)
//				<resource-name>
func migrateFileSystemLayout() error {
	plugins, err := coreutils.GetPluginsDirectoryContent()
	if err != nil {
		return err
	}
	pluginsDir, err := coreutils.GetJfrogPluginsDir()
	for _, p := range plugins {
		if p.IsDir() {
			log.Error("unexpected directory in plugins directory")
			break
		}
		pluginsName := getPluginsNameFromExec(p.Name())
		// TODO: check
		err = fileutils.CopyFile(p.Name(), filepath.Join(pluginsDir, pluginsName, coreutils.PluginsExecDirName, p.Name()))
		if err != nil {
			return err
		}
	}
	return nil
}

func getPluginsNameFromExec(execName string) string {
	return strings.Split(execName, ".")[0]
}

func createPluginsYamlFile() (*PluginsV1, error) {
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
	err = ioutil.WriteFile(pluginsFilePath, content, 0600)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	return plugins, nil
}

func getPluginsFilePath() (string, error) {
	confPath, err := coreutils.GetJfrogHomeDir()
	if err != nil {
		return "", err
	}
	err = os.MkdirAll(confPath, 0777)
	if err != nil {
		return "", err
	}
	confPath = filepath.Join(confPath, coreutils.JfrogPluginsFile)
	return confPath, nil
}
