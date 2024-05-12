package jobsummaries

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils"
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

type MarkdownSection string

// Final markdown sections
// These sections will be inserted into the final markdown file as collapsable sections
const (
	ArtifactsUploadSection MarkdownSection = "upload-data"
	BuildPublishSection    MarkdownSection = "build-publish"
	SecuritySection        MarkdownSection = "security"
)

type JobSummaryInterface interface {
	// AppendResultObject This function should accept a result object, and append it to previous runs of the same objects
	// to allow data persistence between different commands executions.
	AppendResultObject(currentResult interface{}, previousResults []byte) ([]byte, error)
	// RenderContentToMarkdown This function should render the content into a markdown string
	// Notice your markdown will be inserted into collapsable sections in the final markdown file.
	RenderContentToMarkdown(content []byte) (string, error)
	// GetSectionTitle Set section title to inert as collapsable section title
	GetSectionTitle() string
}

type JobSummary struct {
	JobSummaryInterface
	homeDirPath     string // Directory path for the JobSummary data
	platformUrl     string // Platform URL from env,used to generate Markdown links.
	jfrogProjectKey string // [Optional] JFROG_CLI_BUILD_PROJECT env variable
}

const (
	githubActionsEnv  = "GITHUB_ACTIONS"
	JobSummaryDirName = "jfrog-job-summary"
	jfrogHomeDir      = ".jfrog"
	platformUrlEnv    = "JF_URL"
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
		platformUrl:         utils.AddTrailingSlashIfNeeded(os.Getenv(platformUrlEnv)),
		jfrogProjectKey:     os.Getenv(coreutils.Project)}, nil
}

// RecordResult Appends the result to the file system and generates a section markdown file from the accumulated data.
func (js *JobSummary) RecordResult(content any, section MarkdownSection) (err error) {

	if !IsJobSummaryCISupportedRunner() {
		return nil
	}

	previousObjects, err := js.loadPreviousObjectsAsBytes(js.getSectionFileName(section))
	if err != nil {
		return fmt.Errorf("failed to load previous objects: %w", err)
	}

	dataAsBytes, err := js.AppendResultObject(content, previousObjects)
	if err != nil {
		return fmt.Errorf("failed to parase markdown section objects: %w", err)
	}

	if err = js.writeAggregatedDataToFile(dataAsBytes, js.getSectionFileName(section)); err != nil {
		return fmt.Errorf("failed to write aggregated data to file: %w", err)
	}

	markdown, err := js.RenderContentToMarkdown(dataAsBytes)
	if err != nil {
		return fmt.Errorf("failed to render markdown :%w", err)
	}

	if err = js.saveAggregatedSectionMarkdown(markdown, section); err != nil {
		return fmt.Errorf("failed to save markdown to file system")
	}
	return
}

func (js *JobSummary) loadPreviousObjectsAsBytes(fileName string) ([]byte, error) {
	file, cleanUp, err := openFile(fileName)
	defer func() {
		err = cleanUp()
	}()
	if err != nil {
		return nil, err
	}
	return fileutils.ReadFile(file.Name())
}

func (js *JobSummary) writeAggregatedDataToFile(objectAsBytes []byte, dataFileName string) error {
	file, cleanUp, err := openFile(path.Join(js.homeDirPath, dataFileName))
	if err != nil {
		return err
	}
	defer func() {
		err = cleanUp()
	}()
	_, err = file.Write(objectAsBytes)
	return err
}

func (js *JobSummary) saveAggregatedSectionMarkdown(markdown string, section MarkdownSection) (err error) {
	if err != nil {
		return
	}
	file, err := os.OpenFile(path.Join(js.homeDirPath, string(section)+".md"), os.O_CREATE|os.O_WRONLY, 0644)
	defer func() {
		err = file.Close()
	}()
	if err != nil {
		return
	}
	if _, err = file.WriteString(fmt.Sprintf("\n<details open>\n\n<summary>  %s </summary><p></p> \n\n %s \n\n</details>\n", js.GetSectionTitle(), markdown)); err != nil {
		return
	}
	return
}

func (js *JobSummary) getSectionFileName(section MarkdownSection) string {
	return string(section) + "-data.json"
}

func prepareFileSystem() (homeDir string, err error) {
	homeDir, err = getHomeDirPathByOs()
	log.Info("home dirs: ", homeDir)
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

func getHomeDirPathByOs() (homeDir string, err error) {
	//var osBasePath string
	//osString := os.Getenv("RUNNER_OS")
	//if osString == "" {
	//	return "", fmt.Errorf("failed getting machine OS from RUNNER_OS env. Please set env RUNNER_OS & RUNNER_HOMEDIR to enable job summary")
	//}
	//switch osString {
	//case "Windows":
	//	osBasePath = os.Getenv("USERPROFILE")
	//case "Linux", "macOS":
	//	osBasePath = os.Getenv("HOME")
	//case "self-hosted":
	//	osBasePath = os.Getenv("RUNNER_HOMEDIR")
	//default:
	//	return "", fmt.Errorf("unsupported job summary runner OS: %s, supported OS's are: Windows,Linux,MacOS and self-hosted runners", osString)
	//}
	//if osBasePath == "" {
	//	return "", fmt.Errorf("home directory not found in the environment variable. Please set it to according to your operating system enable job summary")
	//}
	runnerTemp := os.Getenv("RUNNER_TEMP")
	return filepath.Join(runnerTemp, jfrogHomeDir, JobSummaryDirName), nil
}

func openFile(filePath string) (*os.File, func() error, error) {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Error("failed to open file at: ", filePath, " error: ", err)
		return nil, nil, err
	}
	return file, file.Close, nil
}

// Check for supported CI runners, currently only GitHub Actions is supported.
func IsJobSummaryCISupportedRunner() bool {
	return os.Getenv(githubActionsEnv) == "true"
}
