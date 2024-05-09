package githubsummaries

import (
	"fmt"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"path"
	"path/filepath"
)

type GithubSummaryInterface interface {
	// Accepts a result object, and append it to previous runs of the same objects
	AppendResultObject(currentResult interface{}, previousResults []byte) ([]byte, error)
	// Renders an array of results objects into markdown.
	// Notice your markdown will be inserted into collapsable sections in the final markdown file.
	RenderContentToMarkdown(content []byte) (string, error)
	// Set section title to the collapsable markdown
	GetSectionTitle() string
}

type GitHubActionSummaryImpl struct {
	GithubSummaryInterface
	homeDirPath       string   // Directory path for the GitHubActionSummaryImpl data
	platformUrl       string   // Platform URL from env,used to generate Markdown links.
	jfrogProjectKey   string   // [Optional] JFROG_CLI_BUILD_PROJECT env variable
	finalMarkdownFile *os.File // Generated markdown file
}

const (
	githubActionsEnv     = "GITHUB_ACTIONS"
	githubSummaryDirName = "jfrog-github-summary"
	jfrogHomeDir         = ".jfrog"
)

type MarkdownSection string

const (
	ArtifactsUploadSection MarkdownSection = "upload-data"
	BuildPublishSection    MarkdownSection = "build-publish"
	SecuritySection        MarkdownSection = "security"
)

func NewGitHubActionSummaryImpl(impl GithubSummaryInterface) *GitHubActionSummaryImpl {
	homedir, err := getHomeDirByOs()
	if err != nil {
		return nil
	}
	return &GitHubActionSummaryImpl{GithubSummaryInterface: impl,
		homeDirPath:       homedir,
		finalMarkdownFile: nil,
		platformUrl:       utils.AddTrailingSlashIfNeeded(os.Getenv("JF_URL")),
		jfrogProjectKey:   os.Getenv("JFROG_CLI_BUILD_PROJECT")}
}

func (ga *GitHubActionSummaryImpl) RecordResult(content any, section MarkdownSection) (err error) {

	if !IsGithubActions() {
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to initiate github summary: %w", err)
	}

	previousObjects, err := ga.loadPreviousObjectsAsBytes(ga.getSectionFileName(section))
	if err != nil {
		return fmt.Errorf("failed to load previous objects: %w", err)
	}

	dataAsBytes, err := ga.AppendResultObject(content, previousObjects)
	if err != nil {
		return fmt.Errorf("failed to parase markdown section objects: %w", err)
	}

	if err = ga.writeAggregatedDataToFile(dataAsBytes, ga.getSectionFileName(section)); err != nil {
		return fmt.Errorf("failed to write aggregated data to file: %w", err)
	}
	var markdown string
	if markdown, err = ga.RenderContentToMarkdown(dataAsBytes); err != nil {
		return fmt.Errorf("failed to render markdown :%w", err)
	}

	if err = ga.saveMarkdownToFileSystem(markdown, section); err != nil {
		return fmt.Errorf("failed to save markdown to file system")
	}

	return
}

func (ga *GitHubActionSummaryImpl) loadPreviousObjectsAsBytes(fileName string) (data []byte, err error) {
	filePath := path.Join(ga.homeDirPath, fileName)
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

func (ga *GitHubActionSummaryImpl) writeAggregatedDataToFile(objectAsBytes []byte, dataFileName string) (err error) {
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

func (ga *GitHubActionSummaryImpl) getSectionFileName(section MarkdownSection) string {
	return string(section) + "-data.json"
}

func (ga *GitHubActionSummaryImpl) ensureHomeDirExists() error {
	if _, err := os.Stat(ga.homeDirPath); os.IsNotExist(err) {
		err = os.MkdirAll(ga.homeDirPath, 0755)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func (ga *GitHubActionSummaryImpl) saveMarkdownToFileSystem(markdown string, section MarkdownSection) (err error) {
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
	if _, err = file.WriteString(fmt.Sprintf("\n<details open>\n\n<summary>  %s </summary> %s </details>\n", ga.GetSectionTitle(), markdown)); err != nil {
		return
	}
	return
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
	homeDir = filepath.Join(osBasePath, jfrogHomeDir, githubSummaryDirName)
	log.Debug("home dir is:", homeDir)
	return
}

func IsGithubActions() bool {
	return os.Getenv(githubActionsEnv) == "true"
}
