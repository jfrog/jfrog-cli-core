package commandsummary

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"path/filepath"
	"strings"
)

// To create a new command summary, the user must implement this interface.
// The GenerateMarkdownFromFiles function should be implemented to generate Markdown from the provided data file paths.
// This involves loading data from the files and converting it into a Markdown string.
type CommandSummaryInterface interface {
	GenerateMarkdownFromFiles(dataFilePaths []string) (finalMarkdown string, err error)
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

// List of allowed directories for searching indexed content
// This should match the Index enum values.
var allowedDirs = map[string]struct{}{
	string(BuildScan):    {},
	string(DockerScan):   {},
	string(BinariesScan): {},
	string(SarifReport):  {},
}

// Each scan result object can be used to generate violations or vulnerabilities.
type ScanResult interface {
	GetViolations() string
	GetVulnerabilities() string
}

// This interface is used to accumulate scan results from different sources and generate a Markdown summary
type ScanResultMarkdownInterface interface {
	BuildScan(filePaths []string) (result ScanResult)
	DockerScan(filePaths []string) (result ScanResult)
	BinaryScan(filePaths []string) (result ScanResult)
	// Default non scanned component view
	GetNonScanned() (nonScanned ScanResult)
}

const (
	// The name of the directory where all the commands summaries will be stored.
	// Inside this directory, each command will have its own directory.
	OutputDirName         = "jfrog-command-summary"
	finalMarkdownFileName = "markdown.md"
	// Filenames formats
	SarifFileFormat   = "*.sarif"
	DataFileFormat    = "*-data"
	NoneScannedResult = "default"
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
	outputDir := os.Getenv(coreutils.SummaryOutputDirPathEnv)
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
	dataFilesPaths, err := cs.GetDataFilesPaths()
	if err != nil {
		return fmt.Errorf("failed to load data files from directory %s, with error: %w", cs.commandsName, err)
	}
	if len(dataFilesPaths) == 0 {
		return nil
	}
	markdown, err := cs.GenerateMarkdownFromFiles(dataFilesPaths)
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
	log.Debug("Recording data with index:", summaryIndex, "and args:", args)
	return cs.recordInternal(data, summaryIndex, args)
}

// Retrieve all the indexed data files in the current command directory.
func GetIndexedDataFilesPaths() (indexedFilePathsMap IndexedFilesMap, err error) {
	basePath := filepath.Join(os.Getenv(coreutils.SummaryOutputDirPathEnv), OutputDirName)
	exists, err := fileutils.IsDirExists(basePath, false)
	if err != nil || !exists {
		return
	}
	return getIndexedFileRecursively(basePath, true)
}

func (cs *CommandSummary) GetDataFilesPaths() ([]string, error) {
	entries, err := os.ReadDir(cs.summaryOutputPath)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	// Exclude markdown files
	var filePaths []string
	for _, entry := range entries {
		if !entry.IsDir() && !strings.HasSuffix(entry.Name(), ".md") {
			filePaths = append(filePaths, filepath.Join(cs.summaryOutputPath, entry.Name()))
		}
	}
	return filePaths, nil
}

func (cs *CommandSummary) recordInternal(data any, args ...interface{}) (err error) {
	// Handle optional extra arguments for recording
	summaryIndex, extraArgs := extractIndexAndArgs(args)
	// Decide on the location of the file and uses SHA1 on the filename to handle possible invalid chars.
	filePath, sha1FileName, err := determineFilePathAndName(cs.summaryOutputPath, summaryIndex, extraArgs)
	if err != nil {
		return err
	}
	// Create the file and write the data to it.
	return cs.saveDataFile(filePath, sha1FileName, data)
}

func (cs *CommandSummary) saveDataFile(filePath, fileName string, data any) (err error) {
	dataAsBytes, err := convertDataToBytes(data)
	if err != nil {
		return errorutils.CheckError(err)
	}
	return createAndWriteToFile(filePath, fileName, dataAsBytes)
}

func (cs *CommandSummary) saveMarkdownFile(markdown string) (err error) {
	data := []byte(markdown)
	return createAndWriteToFile(cs.summaryOutputPath, finalMarkdownFileName, data)
}

// Retrieve all the indexed data files paths in the given directory
func getIndexedFileRecursively(dirPath string, isRoot bool) (nestedFilesMap IndexedFilesMap, err error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	nestedFilesMap = make(map[Index]map[string]string)
	for _, entry := range entries {
		fullPath := filepath.Join(dirPath, entry.Name())
		if entry.IsDir() {
			// Check if the directory is in the allowedDirs list
			_, allowed := allowedDirs[entry.Name()]
			if isRoot || allowed {
				subNestedFilesMap, err := getIndexedFileRecursively(fullPath, false)
				if err != nil {
					return nil, err
				}
				for subDir, files := range subNestedFilesMap {
					nestedFilesMap[subDir] = files
				}
			}
		} else if !isRoot {
			base := filepath.Base(dirPath)
			if nestedFilesMap[Index(base)] == nil {
				nestedFilesMap[Index(base)] = make(map[string]string)
			}
			nestedFilesMap[Index(base)][entry.Name()] = fullPath
		}
	}
	return nestedFilesMap, nil
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
	return os.Getenv(coreutils.SummaryOutputDirPathEnv) != ""
}

func createAndWriteToFile(filePath, fileName string, data []byte) (err error) {
	var fd *os.File
	// If the filename contains a '*' character, it indicates that a random value will be injected to the filename.
	// This is often used for temporary files to avoid name conflicts.
	// However, for indexed content, we usually want to maintain a consistent filename without randomization.
	if strings.Contains(fileName, "*") {
		fd, err = os.CreateTemp(filePath, fileName)
	} else {
		fd, err = os.Create(filepath.Join(filePath, fileName))
	}
	defer func() {
		err = errors.Join(err, errorutils.CheckError(fd.Close()))
	}()
	if err != nil {
		return errorutils.CheckError(err)
	}

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
		return jsonMarshalWithLinks(data)
	}
}

func createDirIfNotExists(homeDir string) error {
	return errorutils.CheckError(os.MkdirAll(homeDir, 0755))
}

func determineFileName(summaryIndex Index, args []string) string {
	// Sarif report should be saved as a random file with .sarif suffix
	if summaryIndex == SarifReport {
		return SarifFileFormat
	}
	// Regular data files should be saved with a random name and a '-data' suffix.
	if len(args) == 0 {
		return DataFileFormat
	}
	// If there are arguments, they should be concatenated with a '-' separator.
	fileName := strings.Join(args, " ")
	// Specific filenames should be converted to sha1 hash to avoid invalid characters.
	return fileNameToSha1(fileName)
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

// Special JSON marshal function that does not escape HTML characters.
func jsonMarshalWithLinks(t interface{}) ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(t)
	return buffer.Bytes(), err
}
