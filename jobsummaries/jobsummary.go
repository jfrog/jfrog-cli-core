package jobsummaries

import (
	"fmt"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"path"
	"path/filepath"
)

// Understanding the functionality of JobSummary
//
// The JobSummary object's role is to accumulate and document data from various command executions.
// It should save the command results in the filesystem, to allow recording of multiple commands executed by the job.
//
// Each time we record a new result, we need to append it to the previous results saved on the file system,
// and generate a markdown from the entire data we collected so far.
//
// The final markdown file is assembled by the setup-cli cleanup function, assembling all the sections.

// Final markdown sections
// These sections will be inserted into the final markdown file as collapsable sections
type MarkdownSection string

const (
	ArtifactsUploadSection MarkdownSection = "upload-data"
	BuildPublishSection    MarkdownSection = "build-publish"
	SecuritySection        MarkdownSection = "security"
)

type JobSummaryInterface interface {
	// This function is responsible for generating a markdown representation of the command.
	// If you want the output to incorporate data from previous command executions,
	// you need to store this data in a location on the file system that won't be cleared between command executions.
	// You can then use this stored data to generate the markdown.
	//
	// The setup-jfrog-cli uses the markdown file produced by this function to create a Job Summary.
	// To ensure the Action can access the output file, you should create the file in the location specified by the
	// JFROG_CLI_JOB_SUMMARY_HOME_DIR environment variable.
	CreateSummaryMarkdown(content any, section MarkdownSection) error
}

type JobSummary struct {
	JobSummaryInterface
	homeDirPath string
}

const (
	githubActionsEnv  = "GITHUB_ACTIONS"
	JobSummaryDirName = "jfrog-job-summary"
	PlatformUrlEnv    = "JF_URL"
	HomeDirPathEnv    = "JFROG_CLI_JOB_SUMMARY_HOME_DIR"
)

// NewJobSummaryImpl Attempt to create a new JobSummary object
// If the runner is not supported JobSummary will return nil
// If the runner does support and fails to initialize, an error will be returned.
func NewJobSummaryImpl(userImplementation JobSummaryInterface) (js *JobSummary, err error) {
	if !IsJobSummaryCISupportedRunner() {
		return nil, nil
	}
	homedir, err := prepareFileSystem()
	if err != nil {
		return nil, fmt.Errorf("failed to prepare file system for job summaries, please check all the env vars are set correctly: %w", err)
	}
	return &JobSummary{
		JobSummaryInterface: userImplementation,
		homeDirPath:         homedir,
	}, nil
}

// This function is designed to be used as a guide for implementing the JobSummaryInterface.
//
// The function takes four parameters:
//   - content: the data that needs to be included in the markdown.
//   - section: the MarkdownSection of the markdown that the content belongs to.
//   - appendObjectsFunc: should handle how the new content is appended to the existing content, if it exists.
//   - generateMarkdownFunc: handles how the data (the result of appendObjectsFunc) is converted into a markdown string.
func CreateSummaryMarkdownBaseImpl(content any, section MarkdownSection, appendObjectsFunc func(interface{}, []byte) ([]byte, error), generateMarkdownFunc func([]byte) (string, error)) (err error) {
	previousObjects, err := loadFile(getSectionDataFileName(section))
	if err != nil {
		return fmt.Errorf("failed to load previous objects: %w", err)
	}

	dataAsBytes, err := appendObjectsFunc(content, previousObjects)
	if err != nil {
		return fmt.Errorf("failed to parase markdown section objects: %w", err)
	}

	if err = writeFile(dataAsBytes, getSectionDataFileName(section)); err != nil {
		return fmt.Errorf("failed to write aggregated data to file: %w", err)
	}

	markdown, err := generateMarkdownFunc(dataAsBytes)
	if err != nil {
		return fmt.Errorf("failed to render markdown :%w", err)
	}

	if err = writeMarkdownToFileSystem(markdown, section); err != nil {
		return fmt.Errorf("failed to save markdown to file system")
	}
	return
}

// Returning the home directory path for the job summaries if set by the user
// Notice that when you set the home directory to make sure it is scoped per job,
// to avoid conflicts between different jobs.
func GetJobSummariesHomeDirPath() (homeDir string, err error) {
	userDefinedHomeDir := os.Getenv(HomeDirPathEnv)
	if userDefinedHomeDir != "" {
		return filepath.Join(userDefinedHomeDir, JobSummaryDirName), nil
	}
	return "", fmt.Errorf("failed to get jobs summaries working dir path, please set JFROG_CLI_JOB_SUMMARY_HOME_DIR")
}

// Check for supported CI runners, currently only GitHub Actions is supported.
func IsJobSummaryCISupportedRunner() bool {
	return os.Getenv(githubActionsEnv) == "true"
}

// Loads a file as bytes array from the file system from the job summaries directory
func loadFile(fileName string) ([]byte, error) {
	homeDir, err := GetJobSummariesHomeDirPath()
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
	homeDir, err := GetJobSummariesHomeDirPath()
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

func writeMarkdownToFileSystem(markdown string, section MarkdownSection) (err error) {
	homedDir, err := GetJobSummariesHomeDirPath()
	if err != nil {
		return
	}
	file, err := os.OpenFile(path.Join(homedDir, string(section)+".md"), os.O_CREATE|os.O_WRONLY, 0644)
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

func getSectionDataFileName(section MarkdownSection) string {
	return string(section) + "-data.json"
}

func prepareFileSystem() (homeDir string, err error) {
	homeDir, err = GetJobSummariesHomeDirPath()
	if err != nil {
		return
	}
	if err = ensureHomeDirExists(homeDir); err != nil {
		return
	}
	return
}

func ensureHomeDirExists(homeDir string) error {
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
