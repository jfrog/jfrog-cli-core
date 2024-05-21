package commandsummary

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type CommandSummaryInterface interface {
	GenerateMarkdownFromFiles(dataFilePaths []string) (finalMarkdown string, err error)
}

const (
	PlatformUrlEnv   = "JF_URL"
	OutputDirName    = "jfrog-command-summary"
	OutputDirPathEnv = "JFROG_CLI_COMMAND_SUMMARY_OUTPUT_DIR"
)

type CommandSummary struct {
	CommandSummaryInterface
	summaryOutputPath string
	commandsName      string
}

func New(userImplementation CommandSummaryInterface, commandsName string) (cs *CommandSummary, err error) {
	if !ArePrerequisitesMet() {
		return nil, nil
	}
	cs = &CommandSummary{
		CommandSummaryInterface: userImplementation,
		commandsName:            commandsName,
	}
	err = cs.prepareFileSystem()
	return
}

// This function stores the current data on the file system.
// It then invokes the GenerateMarkdownFromFiles function on all existing data files.
// Finally, it saves the generated markdown file to the file system.
func (cs *CommandSummary) CreateMarkdown(data any) (err error) {
	if err = cs.saveDataToFileSystem(data); err != nil {
		return
	}
	dataFilesPaths, err := cs.getAllDataFilesPaths()
	if err != nil {
		return fmt.Errorf("failed to load data files from directory %s, with error: %w", cs.commandsName, err)
	}
	markdown, err := cs.GenerateMarkdownFromFiles(dataFilesPaths)
	if err != nil {
		return fmt.Errorf("failed to render markdown: %w", err)
	}
	if err = cs.saveMarkdownToFileSystem(markdown); err != nil {
		return fmt.Errorf("failed to save markdown to file system")
	}
	return
}

func (cs *CommandSummary) getAllDataFilesPaths() ([]string, error) {
	entries, err := os.ReadDir(cs.summaryOutputPath)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	// Exclude markdown files
	var filePaths []string
	for _, entry := range entries {
		if !entry.IsDir() && !strings.HasSuffix(entry.Name(), ".md") {
			filePaths = append(filePaths, path.Join(cs.summaryOutputPath, entry.Name()))
		}
	}
	return filePaths, nil
}

func (cs *CommandSummary) saveMarkdownToFileSystem(markdown string) (err error) {
	file, err := os.OpenFile(path.Join(cs.summaryOutputPath, "markdown.md"), os.O_CREATE|os.O_WRONLY, 0644)
	defer func() {
		err = errors.Join(err, file.Close())
	}()
	if err != nil {
		return errorutils.CheckError(err)
	}
	if _, err = file.WriteString(markdown); err != nil {
		return errorutils.CheckError(err)
	}
	return
}

// Saves the given data into a file in the specified directory.
func (cs *CommandSummary) saveDataToFileSystem(data interface{}) error {
	// Create a random file name in the data file path.
	fd, err := os.CreateTemp(cs.summaryOutputPath, "data-*")
	if err != nil {
		return errorutils.CheckError(err)
	}
	defer func() {
		err = errors.Join(err, fd.Close())
	}()

	// Convert the data into bytes.
	bytes, err := convertDataToBytes(data)
	if err != nil {
		return errorutils.CheckError(err)
	}

	// Write the bytes to the file.
	if _, err = fd.Write(bytes); err != nil {
		return errorutils.CheckError(err)
	}

	return nil
}

func (cs *CommandSummary) prepareFileSystem() (err error) {
	summaryBaseDirPath := filepath.Join(os.Getenv(OutputDirPathEnv), OutputDirName)
	if err = createDirIfNotExists(summaryBaseDirPath); err != nil {
		return errorutils.CheckError(err)
	}
	cs.summaryOutputPath = filepath.Join(summaryBaseDirPath, cs.commandsName)
	if err = createDirIfNotExists(cs.summaryOutputPath); err != nil {
		return errorutils.CheckError(err)
	}
	return
}

func ArePrerequisitesMet() bool {
	homeDirPath := os.Getenv(OutputDirPathEnv)
	return homeDirPath != ""
}

// Helper function to unmarshal data from a file path into the target object.
func UnmarshalFromFilePath(dataFile string, target any) (err error) {
	data, err := fileutils.ReadFile(dataFile)
	if err != nil {
		return
	}
	if err = json.Unmarshal(data, target); err != nil {
		log.Error("Failed to unmarshal data: ", err)
		return errorutils.CheckError(err)
	}
	return
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

func createDirIfNotExists(homeDir string) error {
	return errorutils.CheckError(os.MkdirAll(homeDir, 0755))
}
