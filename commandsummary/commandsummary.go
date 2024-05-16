package commandsummary

import (
	"fmt"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"path"
	"path/filepath"
)

type CommandSummaryInterface interface {
	CreateMarkdown(content any, commandGroup string) error
}

type CommandSummary struct {
	CommandSummaryInterface
	homeDirPath string
}

const (
	githubActionsEnv  = "GITHUB_ACTIONS"
	JobSummaryDirName = "jfrog-job-summary"
	PlatformUrlEnv    = "JF_URL"
	OutputDir         = "JFROG_CLI_COMMAND_SUMMARY_OUTPUT_DIR"
)

func NewCommandSummary(userImplementation CommandSummaryInterface) (js *CommandSummary, err error) {
	if !IsJobSummaryCISupportedRunner() {
		return nil, nil
	}
	homedir, err := prepareFileSystem()
	if err != nil {
		return nil, fmt.Errorf("failed to prepare file system for job summaries, please check all the env vars are set correctly: %w", err)
	}
	return &CommandSummary{
		CommandSummaryInterface: userImplementation,
		homeDirPath:             homedir,
	}, nil
}

func CreateSummaryMarkdownBaseImpl(data any, dirName string, generateMarkdownFunc func(dataFilePaths []string) (finalMarkdown string, mkError error)) (err error) {
	if err = saveDataFile(data, dirName); err != nil {
		return
	}
	markdown, err := generateMarkdownFunc(getDataFilesPaths())
	if err != nil {
		return fmt.Errorf("failed to render markdown :%w", err)
	}
	if err = saveFinalMarkdown(markdown, dirName); err != nil {
		return fmt.Errorf("failed to save markdown to file system")
	}
	return
}

func saveFinalMarkdown(markdown string, name string) error {
	return nil
}

func saveDataFile(data any, dirName string) error {
	getCommandSummariesBaseDirPath()
	return nil
}

func getDataFilesPaths() []string {
	return []string{}
}

// Returning the home directory path for the job summaries if set by the user
// Notice that when you set the home directory to make sure it is scoped per job,
// to avoid conflicts between different jobs.
func getCommandSummariesBaseDirPath() (homeDir string, err error) {
	userDefinedHomeDir := os.Getenv(OutputDir)
	if userDefinedHomeDir != "" {
		return filepath.Join(userDefinedHomeDir, JobSummaryDirName), nil
	}
	return "", fmt.Errorf("failed to get jobs summaries working dir path, please set %s enviorment variable ", OutputDir)
}

// Check for supported CI runners, currently only GitHub Actions is supported.
func IsJobSummaryCISupportedRunner() bool {
	return os.Getenv(githubActionsEnv) == "true"
}

// Loads a file as bytes array from the file system from the job summaries directory
func loadFile(fileName string) ([]byte, error) {
	homeDir, err := getCommandSummariesBaseDirPath()
	if err != nil {
		return nil, err
	}
	file, cleanUp, err := openFile(path.Join(homeDir, fileName))
	defer func() {
		err = cleanUp()
	}()
	if err != nil {
		return nil, err
	}
	return fileutils.ReadFile(file.Name())
}

// Write data to a file as byte array in the job summaries directory
func writeFile(objectAsBytes []byte, dataFileName string) error {
	homeDir, err := getCommandSummariesBaseDirPath()
	if err != nil {
		return err
	}
	file, cleanUp, err := openFile(path.Join(homeDir, dataFileName))
	defer func() {
		err = cleanUp()
	}()
	if err != nil {
		return err
	}
	_, err = file.Write(objectAsBytes)
	return err
}

//func writeMarkdownToFileSystem(markdown string, section MarkdownSection) (err error) {
//	homedDir, err := getCommandSummariesBaseDirPath()
//	if err != nil {
//		return
//	}
//	file, err := os.OpenFile(path.Join(homedDir, string(section)+".md"), os.O_CREATE|os.O_WRONLY, 0644)
//	defer func() {
//		err = file.Close()
//	}()
//	if err != nil {
//		return
//	}
//	if _, err = file.WriteString(markdown); err != nil {
//		return
//	}
//	return
//}
//
//func getSectionDataFileName(section MarkdownSection) string {
//	return string(section) + "-data.json"
//}

func prepareFileSystem() (homeDir string, err error) {
	homeDir, err = getCommandSummariesBaseDirPath()
	if err != nil {
		return
	}
	if err = createDirIfNotExists(homeDir); err != nil {
		return
	}
	return
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

func openFile(filePath string) (*os.File, func() error, error) {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Error("failed to open file at: ", filePath, " error: ", err)
		return nil, nil, err
	}
	return file, file.Close, nil
}
