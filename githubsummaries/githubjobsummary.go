package githubsummaries

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/githubsummariesimpl"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type GithubSummaryInterface interface {
	// Accepts a result object, and append it to previous runs of the same objects
	AppendResultObject(currentResult interface{}, previousResults []byte) ([]byte, error)
	// Renders an array of results objects into markdown.
	// Notice your markdown will be inserted into collapsable sections in the final markdown file.
	RenderContentToMarkdown(content []byte) (string, error)
}

type GitHubActionSummaryImpl struct {
	userMethods       GithubSummaryInterface
	homeDirPath       string   // Directory path for the GitHubActionSummaryImpl data
	platformUrl       string   // Platform URL from env,used to generate Markdown links.
	jfrogProjectKey   string   // [Optional] JFROG_CLI_BUILD_PROJECT env variable
	finalMarkdownFile *os.File // Generated markdown file
}

const (
	githubActionsEnv            = "GITHUB_ACTIONS"
	summaryReadMeFileName       = "summary.md"
	githubSummaryDirName        = "jfrog-github-summary"
	jfrogHomeDir                = ".jfrog"
	artifactsUploadSectionTitle = " 📁 Files uploaded to Artifactory by this job"
	buildPublishSectionTitle    = " 📦 Build Info published to Artifactory by this job"
	SecuritySectionTitle        = " 🔍 Binary scans results by this job"
)

type MarkdownSection string

const (
	ArtifactsUploadSection MarkdownSection = "upload"
	BuildPublishSection    MarkdownSection = "buildPublish"
	SecuritySection        MarkdownSection = "security"
)

func GithubSummaryRecordResult(content any, section MarkdownSection) (err error) {

	if !IsGithubActions() {
		return nil
	}

	ga, err := initiateGithubSummary(section)
	if err != nil {
		return fmt.Errorf("failed to initiate github summary: %w", err)
	}

	previousObjects, err := ga.loadPreviousObjectsAsBytes(ga.getSectionFileName(section))
	if err != nil {
		return fmt.Errorf("failed to load previous objects: %w", err)
	}

	dataAsBytes, err := ga.userMethods.AppendResultObject(content, previousObjects)
	if err != nil {
		return fmt.Errorf("failed to parase markdown section objects: %w", err)
	}

	if err = ga.writeAggregatedDataToFile(dataAsBytes, ga.getSectionFileName(section)); err != nil {
		return fmt.Errorf("failed to write aggregated data to file: %w", err)
	}
	return
}

func (ga *GitHubActionSummaryImpl) loadPreviousObjectsAsBytes(fileName string) (data []byte, err error) {
	runnerHomeDir, err := getHomeDirByOs()
	if err != nil {
		return
	}
	filePath := path.Join(runnerHomeDir, fileName)
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0644)
	defer func() {
		err = file.Close()
	}()
	if err != nil {
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

// TODO move to setup-cli
// TODO remove github from any titles or names
// Change interface
// About project -> remove what i I did and ask about carmit if that's the place
// JF SCAN - JF DOCKER SCAN - JF BUILD-SCAN
func (ga *GitHubActionSummaryImpl) generateMarkdown() (err error) {
	cleanUp, err := ga.createMarkdownFile()
	if err != nil {
		return err
	}
	defer func() {
		err = cleanUp()
	}()
	if err = ga.writeMarkdownHeaders(); err != nil {
		return
	}

	// Artifacts section
	if err = ga.writeArtifactsUploadSection(); err != nil {
		return
	}
	// Build Published section
	if err = ga.writeBuildPublishSection(); err != nil {
		return
	}

	// Security section
	// TODO implement

	return
}

func (ga *GitHubActionSummaryImpl) writeMarkdownHeaders() (err error) {
	if err = ga.writeTitleToMarkdown(); err != nil {
		return
	}
	if err = ga.writeProjectPackagesToMarkdown(); err != nil {
		return
	}
	return
}

func (ga *GitHubActionSummaryImpl) writeArtifactsUploadSection() error {
	uploadCommand := githubsummariesimpl.GithubSummaryRtUploadImpl{
		PlatformUrl:     ga.platformUrl,
		JfrogProjectKey: ga.jfrogProjectKey,
	}
	uploadedArtifactsData := ga.loadDataFileFromSystem(ga.getSectionFileName(ArtifactsUploadSection))
	if uploadedArtifactsData != nil {
		uploadMarkdown, err := uploadCommand.RenderContentToMarkdown(uploadedArtifactsData)
		if err != nil {
			return err
		}
		wrappedSectionMarkdown, err := wrapSectionMarkdown(uploadMarkdown, artifactsUploadSectionTitle)
		if err != nil {
			return err
		}
		if _, err = ga.finalMarkdownFile.WriteString(wrappedSectionMarkdown); err != nil {
			return err
		}
	}
	return nil
}

func (ga *GitHubActionSummaryImpl) writeBuildPublishSection() error {
	buildPublishCommand := githubsummariesimpl.GithubSummaryBpImpl{}
	buildPublishData := ga.loadDataFileFromSystem(ga.getSectionFileName(BuildPublishSection))
	if buildPublishData != nil {
		buildPublishMarkdown, err := buildPublishCommand.RenderContentToMarkdown(buildPublishData)
		if err != nil {
			return err
		}
		wrappedSectionMarkdown, err := wrapSectionMarkdown(buildPublishMarkdown, buildPublishSectionTitle)
		if err != nil {
			return err
		}
		_, err = ga.finalMarkdownFile.WriteString(wrappedSectionMarkdown)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ga *GitHubActionSummaryImpl) createMarkdownFile() (cleanUp func() error, err error) {
	tempMarkdownPath := path.Join(ga.homeDirPath, summaryReadMeFileName)
	if err = os.Remove(tempMarkdownPath); err != nil {
		log.Debug("failed to remove old markdown file: ", err)
	}
	ga.finalMarkdownFile, err = os.OpenFile(tempMarkdownPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	cleanUp = ga.finalMarkdownFile.Close
	return
}

func (ga *GitHubActionSummaryImpl) writeTitleToMarkdown() (err error) {
	return ga.writeStringToMarkdown("<p >\n  <h1> \n    <picture><img src=\"https://github.com/EyalDelarea/jfrog-cli-core/blob/github_job_summary/utils/assests/JFrogLogo.png?raw=true\" style=\"margin: 0 0 -10px 0\"width=\"65px\"></picture> JFrog Platform Job Summary \n     </h1> \n</p>  \n\n")
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

func (ga *GitHubActionSummaryImpl) loadDataFileFromSystem(fileName string) (data []byte) {
	data, _ = fileutils.ReadFile(filepath.Join(ga.homeDirPath, fileName))
	return data
}

func (ga *GitHubActionSummaryImpl) writeStringToMarkdown(str string) error {
	_, err := ga.finalMarkdownFile.WriteString(str)
	if err != nil {
		log.Error(fmt.Errorf("failed to write string to file: %w", err))
		return err
	}
	return nil
}

func (ga *GitHubActionSummaryImpl) writeProjectPackagesToMarkdown() error {
	projectPackagesUrl := fmt.Sprintf("%sui/packages?projectKey=%s", ga.platformUrl, ga.jfrogProjectKey)
	return ga.writeStringToMarkdown(fmt.Sprintf("\n📦 [Project %s packages](%s)\n\n", ga.jfrogProjectKey, projectPackagesUrl))
}

func wrapSectionMarkdown(inputMarkdown, collapseSectionTitle string) (string, error) {
	var markdownBuilder strings.Builder
	if _, err := markdownBuilder.WriteString("<details open>\n"); err != nil {
		return "", err
	}
	if _, err := markdownBuilder.WriteString(fmt.Sprintf("<summary> %s </summary>\n\n\n\n", collapseSectionTitle)); err != nil {
		return "", err
	}
	if _, err := markdownBuilder.WriteString(inputMarkdown); err != nil {
		return "", err
	}
	if _, err := markdownBuilder.WriteString("</details>\n\n"); err != nil {
		return "", err
	}
	return markdownBuilder.String(), nil
}

func initiateGithubSummary(section MarkdownSection) (gh *GitHubActionSummaryImpl, err error) {
	gh, err = newGithubActionSummary(section)
	if err != nil {
		return
	}
	if err = gh.ensureHomeDirExists(); err != nil {
		return nil, err
	}
	return
}

func newGithubActionSummary(section MarkdownSection) (gh *GitHubActionSummaryImpl, err error) {
	homedir, err := getHomeDirByOs()
	if err != nil {
		return
	}
	gh = &GitHubActionSummaryImpl{
		userMethods:       getCommandMethods(section),
		homeDirPath:       homedir,
		finalMarkdownFile: nil,
		platformUrl:       utils.AddTrailingSlashIfNeeded(os.Getenv("JF_URL")),
		jfrogProjectKey:   os.Getenv("JFROG_CLI_PROJECT"),
	}
	return
}

func getCommandMethods(section MarkdownSection) GithubSummaryInterface {
	switch section {
	case ArtifactsUploadSection:
		return &githubsummariesimpl.GithubSummaryRtUploadImpl{}
	case BuildPublishSection:
		return &githubsummariesimpl.GithubSummaryBpImpl{}
	// case Scan:
	//	return &ScanSummary{}
	default:
		return nil
	}
}

func getHomeDirByOs() (homeDir string, err error) {
	osBasePath, err := getBasePathByOs()
	if err != nil {
		return
	}
	homeDir = filepath.Join(osBasePath, jfrogHomeDir, githubSummaryDirName)
	return
}

func getBasePathByOs() (osBasePath string, err error) {
	switch osString := os.Getenv("RUNNER_OS"); osString {
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
	return
}

func triggerMarkdownGeneration(command MarkdownSection) (err error) {
	if !IsGithubActions() {
		return
	}
	gh, err := initiateGithubSummary(command)
	if err != nil {
		return
	}
	return gh.generateMarkdown()
}

func IsGithubActions() bool {
	return os.Getenv(githubActionsEnv) == "true"
}
