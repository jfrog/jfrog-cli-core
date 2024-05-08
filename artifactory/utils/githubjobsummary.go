package utils

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"path"
	"path/filepath"
)

// GithubSummaryInterface The interface for the GitHub Summary implementation
// Users that would like to implement their own GitHub Summary should implement this interface
type GithubSummaryInterface interface {
	convertContentToMarkdown(content []byte) (string, error)
	handleSpecificObject(output interface{}, previousObjects []byte) ([]byte, error)
	getDataFileName() string
}

type GitHubActionSummaryImpl struct {
	userMethods       GithubSummaryInterface
	homeDirPath       string   // Directory path for the GitHubActionSummaryImpl data
	platformUrl       string   // Platform URL from env,used to generate Markdown links.
	jfrogProjectKey   string   // [Optional] JFROG_CLI_PROJECT env variable
	finalMarkdownFile *os.File // Generated markdown file
}

const (
	githubActionsEnv      = "GITHUB_ACTIONS"
	summaryReadMeFileName = "summary.md"
	githubSummaryDirName  = "jfrog-github-summary"
	jfrogHomeDir          = ".jfrog"
)

type MarkdownSection string

const (
	Upload       MarkdownSection = "upload"
	BuildPublish MarkdownSection = "buildPublish"
	Security     MarkdownSection = "security"
)

func (ga *GitHubActionSummaryImpl) RecordCommandOutput(content any, section MarkdownSection) (err error) {
	if !IsGithubActions() {
		return nil
	}
	previousObjects, err := ga.loadPreviousObjectsAsBytes(ga.getDataFileName())
	if err != nil {
		return
	}

	dataAsBytes, err := ga.userMethods.handleSpecificObject(content, previousObjects)
	if err != nil {
		return
	}

	if err = ga.writeAggregatedDataToFile(dataAsBytes, ga.getDataFileName()); err != nil {
		return
	}
	return triggerMarkdownGeneration(section)
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
	// Read the existing data
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

func (ga *GitHubActionSummaryImpl) getDataFileName() string {
	return path.Join(ga.homeDirPath, ga.userMethods.getDataFileName())
}

func (ga *GitHubActionSummaryImpl) generateMarkdown() (err error) {
	cleanUp, err := ga.createMarkdownFile()
	if err != nil {
		return err
	}
	defer func() {
		err = cleanUp()
	}()
	if err = ga.writeTitleToMarkdown(); err != nil {
		return
	}

	if err = ga.writeProjectPackagesToMarkdown(); err != nil {
		return
	}

	// Upload Artifacts Section
	uploadCommand := GithubSummaryRtUploadImpl{}
	uploadedArtifactsData := ga.loadDataFileFromSystem(uploadCommand.getDataFileName())
	uploadMarkdown, err := uploadCommand.convertContentToMarkdown(uploadedArtifactsData)
	_, err = ga.finalMarkdownFile.WriteString(uploadMarkdown)

	// Build Publish Section
	buildPublishCommand := GithubSummaryBpImpl{}
	buildPublishData := ga.loadDataFileFromSystem(buildPublishCommand.getDataFileName())
	buildPublishMarkdown, err := buildPublishCommand.convertContentToMarkdown(buildPublishData)
	_, err = ga.finalMarkdownFile.WriteString(buildPublishMarkdown)

	// Security section

	return
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

func (ga *GitHubActionSummaryImpl) buildUiUrl(targetPath string) string {
	template := "%sui/repos/tree/General/%s/?projectKey=%s"
	return fmt.Sprintf(template, ga.platformUrl, targetPath, ga.jfrogProjectKey)
}

func (ga *GitHubActionSummaryImpl) createTempFileIfNeeded(filePath string, content any) (err error) {
	exists, err := fileutils.IsFileExists(filePath, true)
	if err != nil || exists {
		return
	}
	file, err := os.Create(filePath)
	defer func() {
		err = file.Close()
	}()
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	bytes, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("failed to marshal content: %w", err)
	}
	_, err = file.Write(bytes)
	if err != nil {
		return fmt.Errorf("failed to write content: %w", err)
	}
	log.Info("created file:", file.Name())
	return
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
	case Upload:
		return &GithubSummaryRtUploadImpl{}
	case BuildPublish:
		return &GithubSummaryBpImpl{}
	//case Scan:
	//	return &ScanSummary{}
	default:
		return nil
	}
}

func IsGithubActions() bool {
	return os.Getenv(githubActionsEnv) == "true"
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

// Called after each record function to trigger the markdown generation.
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
