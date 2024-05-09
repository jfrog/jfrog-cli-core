package jobsummaries

import (
	"fmt"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"path"
	"path/filepath"
)

type MarkdownSection string

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
	finalMarkdownFile *os.File // Generated markdown file
	homeDirPath       string   // Directory path for the JobSummary data
	platformUrl       string   // Platform URL from env,used to generate Markdown links.
	jfrogProjectKey   string   // [Optional] JFROG_CLI_BUILD_PROJECT env variable
}

const (
	githubActionsEnv  = "GITHUB_ACTIONS"
	JobSummaryDirName = "jfrog-job-summary"
	jfrogHomeDir      = ".jfrog"
)

func NewJobSummaryImpl(userImplementation JobSummaryInterface) *JobSummary {
	homedir, err := getHomeDirByOs()
	if err != nil {
		return nil
	}
	if err = ensureHomeDirExists(homedir); err != nil {
		return nil
	}
	return &JobSummary{
		JobSummaryInterface: userImplementation,
		homeDirPath:         homedir,
		finalMarkdownFile:   nil,
		platformUrl:         utils.AddTrailingSlashIfNeeded(os.Getenv("JF_URL")),
		jfrogProjectKey:     os.Getenv("JFROG_CLI_BUILD_PROJECT")}
}

func (js *JobSummary) RecordResult(content any, section MarkdownSection) (err error) {

	if !IsGithubActions() {
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

	if err = js.saveMarkdownToFileSystem(markdown, section); err != nil {
		return fmt.Errorf("failed to save markdown to file system")
	}

	return
}

func (js *JobSummary) loadPreviousObjectsAsBytes(fileName string) (data []byte, err error) {
	filePath := path.Join(js.homeDirPath, fileName)
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0644)
	defer func() {
		err = file.Close()
	}()
	if err != nil {
		log.Error("failed to open file at: ", filePath, " error: ", err)
		return
	}
	return fileutils.ReadFile(filePath)
}

func (js *JobSummary) writeAggregatedDataToFile(objectAsBytes []byte, dataFileName string) (err error) {
	// Marshal the updated array back into JSON
	runnerHomeDir, err := getHomeDirByOs()
	if err != nil {
		return
	}
	filePath := path.Join(runnerHomeDir, dataFileName)
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0644)
	defer func() {
		err = file.Close()
	}()
	_, err = file.Write(objectAsBytes)
	return
}

func (js *JobSummary) getSectionFileName(section MarkdownSection) string {
	return string(section) + "-data.json"
}

func (js *JobSummary) saveMarkdownToFileSystem(markdown string, section MarkdownSection) (err error) {
	homedir, err := getHomeDirByOs()
	if err != nil {
		return
	}
	file, err := os.OpenFile(path.Join(homedir, string(section)+".md"), os.O_CREATE|os.O_WRONLY, 0644)
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

func getHomeDirByOs() (homeDir string, err error) {
	var osBasePath string
	osString := os.Getenv("RUNNER_OS")
	switch osString {
	case "Windows":
		osBasePath = os.Getenv("USERPROFILE")
	case "Linux", "macOS":
		osBasePath = os.Getenv("HOME")
	case "self-hosted":
		osBasePath = os.Getenv("RUNNER_HOMEDIR")
		if osBasePath == "" {
			log.Error("Home directory not found in the environment variable: RUNNER_HOMEDIR, please set it to enable GitHub Job Summary on a self hosted machine")
			err = fmt.Errorf("home directory not found in the environment variable: RUNNER_HOMEDIR, please set it to enable GitHub Job Summary on a self hosted machine")
			return
		}
	default:
		log.Error("Unsupported OS: ", osString)
		err = fmt.Errorf("unsupported OS: %s, supported OS's are: Windows,Linux,MacOS and self-hosted runners", osString)
		return
	}
	if err != nil {
		return
	}
	homeDir = filepath.Join(osBasePath, jfrogHomeDir, JobSummaryDirName)
	return
}

func IsGithubActions() bool {
	return os.Getenv(githubActionsEnv) == "true"
}
