package commandsummary

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"time"
)

type CommandSummaryInterface interface {
	CreateMarkdown(content any) error
}

type CommandSummary struct {
	CommandSummaryInterface
	outputDirPath string
}

const (
	PlatformUrlEnv   = "JF_URL"
	OutputDirName    = "jfrog-command-summary"
	OutputDirPathEnv = "JFROG_CLI_COMMAND_SUMMARY_OUTPUT_DIR"
)

func NewCommandSummary(userImplementation CommandSummaryInterface) (js *CommandSummary, err error) {
	if !ArePrerequisitesMet() {
		return nil, nil
	}
	if err = prepareFileSystem(); err != nil {
		return
	}
	return &CommandSummary{
		CommandSummaryInterface: userImplementation,
		outputDirPath:           getOutputDirPath(),
	}, nil
}

func CreateMarkdown(data any, subDir string, generateMarkdownFunc func(dataFilePaths []string) (finalMarkdown string, mkError error)) (err error) {
	if err = saveDataFile(data, subDir); err != nil {
		return
	}
	dataFilesPaths, err := getDataFilesPaths(subDir)
	if err != nil {
		return fmt.Errorf("failed to load data files from direcoty %s, with error:%w ", subDir, err)
	}
	markdown, err := generateMarkdownFunc(dataFilesPaths)
	if err != nil {
		return fmt.Errorf("failed to render markdown :%w", err)
	}
	if err = saveMarkdownFile(markdown, subDir); err != nil {
		return fmt.Errorf("failed to save markdown to file system")
	}
	return
}

func saveMarkdownFile(markdown string, subDir string) (err error) {
	file, err := os.OpenFile(path.Join(getOutputDirPath(), subDir, "markdown.md"), os.O_CREATE|os.O_WRONLY, 0644)
	defer func() {
		err = file.Close()
	}()
	if err != nil {
		return
	}
	if _, err = file.WriteString(markdown); err != nil {
		return
	}
	return
}

func saveDataFile(data any, dirName string) (err error) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	dataFilePath := path.Join(getOutputDirPath(), dirName)
	if err = createDirIfNotExists(dataFilePath); err != nil {
		return
	}
	fd, err := os.CreateTemp(dataFilePath, dirName+"-"+timestamp+"-")
	defer func() {
		err = fd.Close()
	}()
	bytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	if _, err = fd.Write(bytes); err != nil {
		return
	}
	return
}

func getDataFilesPaths(subDir string) (filePaths []string, err error) {
	subDirFullPath := path.Join(getOutputDirPath(), subDir)
	return GetAllFilePaths(subDirFullPath)
}

func GetAllFilePaths(dirPath string) ([]string, error) {
	var filePaths []string
	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			filePaths = append(filePaths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return filePaths, nil
}

func getOutputDirPath() (homeDir string) {
	// We know OutputDirPathEnv is defined as we checked it in ArePrerequisitesMet
	userDefinedHomeDir := os.Getenv(OutputDirPathEnv)
	return filepath.Join(userDefinedHomeDir, OutputDirName)
}

func ArePrerequisitesMet() bool {
	homeDirPath := os.Getenv(OutputDirPathEnv)
	if homeDirPath == "" {
		return false
	}
	return true
}

func prepareFileSystem() (err error) {
	outputPath := getOutputDirPath()
	return createDirIfNotExists(outputPath)
}

func createDirIfNotExists(homeDir string) error {
	if _, err := os.Stat(homeDir); os.IsNotExist(err) {
		err = os.MkdirAll(homeDir, 0755)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func UnmarshalDataFiles(dataFiles []string, target interface{}) error {
	targetVal := reflect.ValueOf(target)
	if targetVal.Kind() != reflect.Ptr || targetVal.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("target must be a pointer to a slice")
	}
	targetVal = targetVal.Elem()

	for _, dataFile := range dataFiles {
		data, err := fileutils.ReadFile(dataFile)
		if err != nil {
			return err
		}

		elemType := targetVal.Type().Elem()
		elem := reflect.New(elemType)

		if err = json.Unmarshal(data, elem.Interface()); err != nil {
			log.Error("Failed to unmarshal data: ", err)
			return err
		}

		targetVal.Set(reflect.Append(targetVal, elem.Elem()))
	}

	return nil
}

func UnmarshalFromFilePath(dataFile string, target any) (err error) {
	data, err := fileutils.ReadFile(dataFile)
	if err != nil {
		return
	}
	if err = json.Unmarshal(data, target); err != nil {
		log.Error("Failed to unmarshal data: ", err)
		return
	}
	return
}
