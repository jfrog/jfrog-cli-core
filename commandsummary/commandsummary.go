package commandsummary

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
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

// This function stores the current data on the file system.
// It then invokes the generateMarkdown function on all existing data files.
// Finally, it saves the generated markdown file to the file system.
func CreateMarkdown(data any, subDir string, generateMarkdownFunc func(dataFilePaths []string) (finalMarkdown string, mkError error)) (err error) {
	if err = saveDataToFileSystem(data, subDir); err != nil {
		return
	}
	dataFilesPaths, err := GetAllDataFilesPaths(path.Join(getOutputDirPath(), subDir))
	if err != nil {
		return fmt.Errorf("failed to load data files from direcoty %s, with error:%w ", subDir, err)
	}
	markdown, err := generateMarkdownFunc(dataFilesPaths)
	if err != nil {
		return fmt.Errorf("failed to render markdown :%w", err)
	}
	if err = saveMarkdownToFileSystem(markdown, subDir); err != nil {
		return fmt.Errorf("failed to save markdown to file system")
	}
	return
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

func ArePrerequisitesMet() bool {
	homeDirPath := os.Getenv(OutputDirPathEnv)
	return homeDirPath != ""
}

func GetAllDataFilesPaths(dirPath string) ([]string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	// Exclude markdown files
	var filePaths []string
	for _, entry := range entries {
		if !entry.IsDir() && !strings.HasSuffix(entry.Name(), ".md") {
			filePaths = append(filePaths, path.Join(dirPath, entry.Name()))
		}
	}
	return filePaths, nil
}

func saveMarkdownToFileSystem(markdown string, subDir string) (err error) {
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

// Saves the given data into a file in the specified directory.
func saveDataToFileSystem(data interface{}, dirName string) error {

	dataFilePath := path.Join(getOutputDirPath(), dirName)

	if err := createDirIfNotExists(dataFilePath); err != nil {
		return err
	}

	// Create a random file name in the data file path.
	fd, err := os.CreateTemp(dataFilePath, generateRandomFileName(dirName))
	if err != nil {
		return err
	}
	defer func() {
		err = fd.Close()
	}()

	// Convert the data into bytes.
	bytes, err := convertDataToBytes(data)
	if err != nil {
		return err
	}

	// Write the bytes to the file.
	if _, err = fd.Write(bytes); err != nil {
		return err
	}

	return nil
}

// Converts the given data into a byte array.
// Handle specific conversion cases
func convertDataToBytes(data interface{}) ([]byte, error) {
	switch v := data.(type) {
	case []byte:
		return v, nil
	default:
		return json.Marshal(data)
	}
}

func generateRandomFileName(dirName string) string {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	return dirName + "-" + timestamp + "-"
}

// We know OutputDirPathEnv is defined as we checked it in ArePrerequisitesMet
func getOutputDirPath() (homeDir string) {
	return filepath.Join(os.Getenv(OutputDirPathEnv), OutputDirName)
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
