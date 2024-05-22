package commandsummary

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type CommandSummaryInterface interface {
	GenerateMarkdownFromFiles(dataFilePaths []string) (finalMarkdown string, err error)
}

const (
	// The name of the directory where all the commands summaries will be stored.
	// Inside this directory, each command will have its own directory.
	OutputDirName = "jfrog-command-summary"
)

type CommandSummary struct {
	CommandSummaryInterface
	summaryOutputPath string
	commandsName      string
}

// Create a new instance of CommandSummary.
// Notice to check if the command should record the summary before calling this function.
// You can do this by calling the helper function ShouldRecordSummary.
func New(userImplementation CommandSummaryInterface, commandsName string) (cs *CommandSummary, err error) {
	outputDir := os.Getenv(coreutils.OutputDirPathEnv)
	if outputDir == "" {
		return nil, fmt.Errorf("output dir path is not defined,please set the JFROG_CLI_COMMAND_SUMMARY_OUTPUT_DIR environment variable")
	}
	cs = &CommandSummary{
		CommandSummaryInterface: userImplementation,
		commandsName:            commandsName,
		summaryOutputPath:       outputDir,
	}
	err = cs.prepareFileSystem()
	return
}

// This function stores the current data on the file system.
// It then invokes the GenerateMarkdownFromFiles function on all existing data files.
// Finally, it saves the generated markdown file to the file system.
func (cs *CommandSummary) Record(data any) (err error) {
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
		return fmt.Errorf("failed to save markdown to file system: %w", err)
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
	if err != nil {
		return errorutils.CheckError(err)
	}
	defer func() {
		err = errors.Join(err, errorutils.CheckError(file.Close()))
	}()
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

// This function creates the base dir for the command summary inside
// the path the user has provided, userPath/OutputDirName.
// Then it creates a specific directory for the command, path/OutputDirName/commandsName.
// And set the summaryOutputPath to the specific command directory.
func (cs *CommandSummary) prepareFileSystem() (err error) {
	summaryBaseDirPath := filepath.Join(cs.summaryOutputPath, OutputDirName)
	if err = createDirIfNotExists(summaryBaseDirPath); err != nil {
		return err
	}
	specificCommandOutputPath := filepath.Join(summaryBaseDirPath, cs.commandsName)
	if err = createDirIfNotExists(specificCommandOutputPath); err != nil {
		return err
	}
	// Sets the specific command output path
	cs.summaryOutputPath = specificCommandOutputPath
	return
}

// If the output dir path is not defined, the command summary should not be recorded.
func ShouldRecordSummary() bool {
	return os.Getenv(coreutils.OutputDirPathEnv) != ""
}

// Helper function to unmarshal data from a file path into the target object.
func UnmarshalFromFilePath(dataFile string, target any) (err error) {
	data, err := fileutils.ReadFile(dataFile)
	if err != nil {
		return
	}
	if err = json.Unmarshal(data, target); err != nil {
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
