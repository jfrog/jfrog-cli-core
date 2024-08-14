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
	GenerateMarkdownFromFiles(dataFilePaths []string, nestedFilePaths map[SummariesSubDirs]map[string]string) (finalMarkdown string, err error)
}

type SummariesSubDirs string

const (
	Binaries  SummariesSubDirs = "Binaries"
	BuildScan SummariesSubDirs = "Build-Scan"
	Docker    SummariesSubDirs = "Docker"
	Sarif     SummariesSubDirs = "Sarif"
)

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

func (cs *CommandSummary) GenerateMarkdown() error {
	dataFilesPaths, nestedFiles, err := cs.getAllDataFilesPaths()
	if err != nil {
		return fmt.Errorf("failed to load data files from directory %s, with error: %w", cs.commandsName, err)
	}
	markdown, err := cs.GenerateMarkdownFromFiles(dataFilesPaths, nestedFiles)
	if err != nil {
		return fmt.Errorf("failed to render markdown: %w", err)
	}
	if err = cs.saveMarkdownToFileSystem(markdown); err != nil {
		return fmt.Errorf("failed to save markdown to file system: %w", err)
	}
	return nil
}

// This function stores the current data on the file system.
func (cs *CommandSummary) Record(data any) (err error) {
	return cs.recordInternal(data)
}

// This function stores the current data on the file system with additional context.
func (cs *CommandSummary) RecordWithArgs(data any, subDir SummariesSubDirs, args ...string) (err error) {
	return cs.recordInternal(data, subDir, args)
}

func (cs *CommandSummary) recordInternal(data any, args ...interface{}) (err error) {
	var subDir SummariesSubDirs
	var extraArgs []string

	if len(args) > 0 {
		if dir, ok := args[0].(SummariesSubDirs); ok {
			subDir = dir
			if len(args) > 1 {
				extraArgs = args[1].([]string)
			}
		} else {
			extraArgs = args[0].([]string)
		}
	}

	filePath, fileName, err := cs.determineFilePathAndName(subDir, extraArgs)
	if err != nil {
		return err
	}
	return cs.createAndWriteToFile(filePath, fileName, data)
}

func (cs *CommandSummary) determineFilePathAndName(subDir SummariesSubDirs, args []string) (filePath, fileName string, err error) {
	filePath = cs.summaryOutputPath
	if subDir != "" {
		filePath = path.Join(filePath, string(subDir))
		if err = createDirIfNotExists(filePath); err != nil {
			return "", "", err
		}
	}
	if len(args) > 0 {
		fileName = strings.Join(args, "-")
	} else {
		fileName = "data-*"
	}
	return filePath, fileName, nil
}

func (cs *CommandSummary) createAndWriteToFile(filePath, fileName string, data any) (err error) {
	var fd *os.File
	if fileName == "data-*" {
		fd, err = os.CreateTemp(filePath, fileName)
	} else {
		fd, err = os.Create(path.Join(filePath, fileName))
	}
	if err != nil {
		return errorutils.CheckError(err)
	}
	defer func() {
		err = errors.Join(err, fd.Close())
	}()

	bytes, err := convertDataToBytes(data)
	if err != nil {
		return errorutils.CheckError(err)
	}

	if _, err = fd.Write(bytes); err != nil {
		return errorutils.CheckError(err)
	}

	return nil
}
func (cs *CommandSummary) getAllDataFilesPaths() (currentDirFiles []string, nestedFilesMap map[SummariesSubDirs]map[string]string, err error) {
	return cs.getAllDataFilesPathsRecursive(cs.summaryOutputPath, true)
}

func (cs *CommandSummary) getAllDataFilesPathsRecursive(dirPath string, isRoot bool) (currentDirFiles []string, nestedFilesMap map[SummariesSubDirs]map[string]string, err error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, nil, errorutils.CheckError(err)
	}

	nestedFilesMap = make(map[SummariesSubDirs]map[string]string)
	for _, entry := range entries {
		fullPath := path.Join(dirPath, entry.Name())
		if entry.IsDir() {
			_, subNestedFilesMap, err := cs.getAllDataFilesPathsRecursive(fullPath, false)
			if err != nil {
				return nil, nil, err
			}
			for subDir, files := range subNestedFilesMap {
				nestedFilesMap[subDir] = files
			}
		} else if !strings.HasSuffix(entry.Name(), ".md") {
			if isRoot {
				currentDirFiles = append(currentDirFiles, fullPath)
			} else {
				base := path.Base(dirPath)
				if nestedFilesMap[SummariesSubDirs(base)] == nil {
					nestedFilesMap[SummariesSubDirs(base)] = make(map[string]string)
				}
				nestedFilesMap[SummariesSubDirs(base)][entry.Name()] = fullPath
			}
		}
	}
	return currentDirFiles, nestedFilesMap, nil
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
