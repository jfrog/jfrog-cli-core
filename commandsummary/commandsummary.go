package commandsummary

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// To create a new command summary, the user must implement this interface.
// The GenerateMarkdownFromFiles function should be implemented to generate Markdown from the provided data file paths.
// This involves loading data from the files and converting it into a Markdown string.
// The indexedFilePaths map contains special indexed file paths for more advanced use cases,
// see RecordWithIndex comments for more information.
type CommandSummaryInterface interface {
	GenerateMarkdownFromFiles(dataFilePaths []string, indexedFilePaths map[Index]map[string]string) (finalMarkdown string, err error)
}

// These optional index determine where files are saved, making them easier to locate.
// Each category corresponds to a nested folder within the current command summary structure.
//
// For example, if the command summary is for the build-info command and the category is "DockerScan,"
// the file will be saved in the following path: outputDirPath/jfrog-command-summary/build-info/Docker-Scan
type Index string

const (
	BinariesScan Index = "binaries-scans"
	BuildScan    Index = "build-scans"
	DockerScan   Index = "docker-scans"
	SarifReport  Index = "sarif-reports"
)

const (
	// The name of the directory where all the commands summaries will be stored.
	// Inside this directory, each command will have its own directory.
	OutputDirName         = "jfrog-command-summary"
	finalMarkdownFileName = "markdown.md"
	MarkdownSuffix        = ".md"
	// Filenames formats
	SarifFileFormat = "*.sarif"
	DataFileFormat  = "*-data"
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

// Loads all the relevant data files and invoke the implementation to generate the Markdown.
func (cs *CommandSummary) GenerateMarkdown() error {
	dataFilesPaths, indexedFiles, err := cs.getDataFilesPaths()
	if err != nil {
		return fmt.Errorf("failed to load data files from directory %s, with error: %w", cs.commandsName, err)
	}
	markdown, err := cs.GenerateMarkdownFromFiles(dataFilesPaths, indexedFiles)
	if err != nil {
		return fmt.Errorf("failed to render markdown: %w", err)
	}
	if err = cs.saveMarkdownFile(markdown); err != nil {
		return fmt.Errorf("failed to save markdown to file system: %w", err)
	}
	return nil
}

// This function stores the current data on the file system.
func (cs *CommandSummary) Record(data any) (err error) {
	return cs.recordInternal(data)
}

// The RecordWithIndex function saves data into an indexed folder within the command summary directory.
// This allows you to associate specific indexed data with other recorded data using a key-value mapping.
// For example,
// when you have uploaded artifact and want to combine it with its scan results recorded at a different time,
// recording the scan results as an index helps merge the information later on.
//
// Data: The data to be recorded.
// SummaryIndex: The name of the index under which the data will be stored.
// Args: Additional arguments used to determine the file name.
func (cs *CommandSummary) RecordWithIndex(data any, summaryIndex Index, args ...string) (err error) {
	return cs.recordInternal(data, summaryIndex, args)
}

func (cs *CommandSummary) recordInternal(data any, args ...interface{}) (err error) {
	// Handle optional extra arguments for recording
	summaryIndex, extraArgs := extractIndexAndArgs(args)
	// Decide on the location and the file name based on the subject and the extra arguments.
	filePath, fileName, err := determineFilePathAndName(cs.summaryOutputPath, summaryIndex, extraArgs)
	if err != nil {
		return err
	}
	// Create the file and write the data to it.
	return cs.saveDataFile(filePath, fileName, data)
}

func (cs *CommandSummary) saveDataFile(filePath, fileName string, data any) (err error) {
	bytes, err := convertDataToBytes(data)
	if err != nil {
		return errorutils.CheckError(err)
	}
	return createAndWriteToFile(filePath, fileName, bytes)
}

func (cs *CommandSummary) saveMarkdownFile(markdown string) (err error) {
	data := []byte(markdown)
	return createAndWriteToFile(cs.summaryOutputPath, finalMarkdownFileName, data)
}

// Returns all the data files paths in the current command summary directory and nested indexed directories if exists.
func (cs *CommandSummary) getDataFilesPaths() (currentDirFiles []string, nestedFilesMap map[Index]map[string]string, err error) {
	return cs.getAllDataFilesPathsRecursive(cs.summaryOutputPath, true)
}

// Retrieve all the data files paths in the given directory and its subdirectories recursively.
func (cs *CommandSummary) getAllDataFilesPathsRecursive(dirPath string, isRoot bool) (currentDirFiles []string, nestedFilesMap map[Index]map[string]string, err error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, nil, errorutils.CheckError(err)
	}
	nestedFilesMap = make(map[Index]map[string]string)
	for _, entry := range entries {
		fullPath := filepath.Join(dirPath, entry.Name())
		if entry.IsDir() {
			_, subNestedFilesMap, err := cs.getAllDataFilesPathsRecursive(fullPath, false)
			if err != nil {
				return nil, nil, err
			}
			for subDir, files := range subNestedFilesMap {
				nestedFilesMap[subDir] = files
			}
		} else {
			if isRoot {
				currentDirFiles = append(currentDirFiles, fullPath)
			} else {
				base := filepath.Base(dirPath)
				if nestedFilesMap[Index(base)] == nil {
					nestedFilesMap[Index(base)] = make(map[string]string)
				}
				nestedFilesMap[Index(base)][entry.Name()] = fullPath
			}
		}
	}
	return currentDirFiles, nestedFilesMap, nil
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

func createAndWriteToFile(filePath, fileName string, data []byte) (err error) {
	var fd *os.File
	// Create a file
	if strings.Contains(fileName, "*") {
		fd, err = os.CreateTemp(filePath, fileName)
	} else {
		fd, err = os.Create(filepath.Join(filePath, fileName))
	}
	if err != nil {
		return errorutils.CheckError(err)
	}
	defer func() {
		err = errors.Join(err, errorutils.CheckError(fd.Close()))
	}()

	// Write to file
	if _, err = fd.Write(data); err != nil {
		return errorutils.CheckError(err)
	}
	return
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

// File name should be decided based on the subject and args.
func determineFileName(summaryIndex Index, args []string) string {
	if summaryIndex == SarifReport {
		return SarifFileFormat
	}
	if len(args) > 0 {
		fileName := strings.Join(args, "-")
		// If running on Windows, replace backslashes with dashes.
		if runtime.GOOS == "windows" {
			fileName = strings.ReplaceAll(fileName, "\\", "-")
		}
		// Replace all other invalid characters with dashes.
		invalidChars := regexp.MustCompile(`[<>:"/\\|?*]`)
		fileName = invalidChars.ReplaceAllString(fileName, "-")
		return fileName
	}
	return DataFileFormat
}

func determineFilePathAndName(summaryOutputPath string, index Index, args []string) (filePath, fileName string, err error) {
	filePath = summaryOutputPath
	// Create subdirectory if the index is not empty
	if index != "" {
		filePath = filepath.Join(filePath, string(index))
		if err = createDirIfNotExists(filePath); err != nil {
			return "", "", err
		}
	}
	fileName = determineFileName(index, args)
	return
}

func extractIndexAndArgs(args []interface{}) (Index, []string) {
	var index Index
	var extraArgs []string

	if len(args) > 0 {
		if dir, ok := args[0].(Index); ok {
			index = dir
			if len(args) > 1 {
				if extraArgs, ok = args[1].([]string); !ok {
					return index, nil
				}
			}
		} else if extraArgs, ok := args[0].([]string); ok {
			return index, extraArgs
		}
	}
	return index, extraArgs
}
