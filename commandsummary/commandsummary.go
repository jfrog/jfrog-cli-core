package commandsummary

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"
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

func CreateMarkdown(data any, subDir string, generateMarkdownFunc func(dataFilePaths []string) (finalMarkdown string, mkError error)) (err error) {
	if err = saveDataFile(data, subDir); err != nil {
		return
	}
	dataFilesPaths, err := getDataFilesPaths(subDir)
	if err != nil {
		return
	}
	markdown, err := generateMarkdownFunc(dataFilesPaths)
	if err != nil {
		return fmt.Errorf("failed to render markdown :%w", err)
	}
	if err = saveFinalMarkdown(markdown, subDir); err != nil {
		return fmt.Errorf("failed to save markdown to file system")
	}
	return
}

func saveFinalMarkdown(markdown string, subDir string) (err error) {
	baseDir, err := getCommandSummariesBaseDirPath()
	if err != nil {
		return
	}
	file, err := os.OpenFile(path.Join(baseDir, subDir, "markdown.md"), os.O_CREATE|os.O_WRONLY, 0644)
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
	baseDir, err := getCommandSummariesBaseDirPath()
	if err != nil {
		return
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	saveTo := path.Join(baseDir, dirName)
	if err = createDirIfNotExists(saveTo); err != nil {
		return
	}
	fd, err := os.CreateTemp(saveTo, dirName+"-"+timestamp+"-")
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
	baseDir, err := getCommandSummariesBaseDirPath()
	if err != nil {
		return
	}
	subDirFullPath := path.Join(baseDir, subDir)
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
