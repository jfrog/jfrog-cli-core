package utils

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"path"
	"path/filepath"
	"time"
)

func (ga *GitHubActionSummaryImpl) RecordCommandOutput(content any) (err error) {
	if !isGitHubActionsRunner() {
		return nil
	}
	fileName := ga.getDataFileName()
	previousObjects, err := ga.loadPreviousObjectsAsBytes(fileName)
	if err != nil {
		return
	}

	dataAsBytes, err := ga.userMethods.handleSpecificObject(content, previousObjects)
	if err != nil {
		return
	}

	return ga.writeAggregatedDataToFile(dataAsBytes, ga.getDataFileName())
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

func (ga *GitHubActionSummaryImpl) convertContentToMarkdown() (markdown string, err error) {
	data, err := ga.loadDataFileFromSystem()
	if err != nil {
		return
	}
	return ga.userMethods.convertContentToMarkdown(data)
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

// GithubSummaryInterface The interface for the GitHub Summary implementation
// Users that would like to implement their own GitHub Summary should implement this interface
type GithubSummaryInterface interface {
	convertContentToMarkdown(content []byte) (string, error)
	handleSpecificObject(output interface{}, previousObjects []byte) ([]byte, error)
	getDataFileName() string
}

type UploadResult struct {
	SourcePath string `json:"sourcePath"`
	TargetPath string `json:"targetPath"`
	RtUrl      string `json:"rtUrl"`
}

type ResultsWrapper struct {
	Results []UploadResult `json:"results"`
}

type GitHubActionSummaryImpl struct {
	userMethods            GithubSummaryInterface
	homeDirPath            string    // Directory path for the GitHubActionSummaryImpl data
	rawUploadArtifactsFile string    // File which contains all the results of the commands
	platformUrl            string    // Platform URL from env,used to generate Markdown links.
	jfrogProjectKey        string    // [Optional] JFROG_CLI_PROJECT env variable
	uploadTree             *FileTree // Upload a tree object to generate markdown
	finalMarkdownFile      *os.File  // Generated markdown file
}

const (
	githubActionsEnv          = "GITHUB_ACTIONS"
	uploadedArtifactsFileName = "uploaded-artifacts-data.json"
	summaryReadMeFileName     = "summary.md"
	githubSummaryDirName      = "jfrog-github-summary"
	jfrogHomeDir              = ".jfrog"
)

// GenerateGitHubActionSummary called by the CLI, it will aggregate the markdown as if
// this current command is that last command in the workflow.
// TODO maybe change name
func GenerateGitHubActionSummary(contentReader *content.ContentReader) (err error) {
	if !isGitHubActionsRunner() {
		return
	}
	gh, err := initiateGithubSummary()
	if err != nil {
		return
	}
	// If content reader is not nil, append the results to the data file.
	if contentReader != nil {
		if err = gh.generateUploadArtifactsTree(contentReader); err != nil {
			return
		}
	}
	return gh.generateMarkdown()
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
	// Upload artifacts section
	if err = ga.writeUploadedArtifactsToMarkdown(); err != nil {
		return
	}
	// Build info section
	if err = ga.buildInfoSection(); err != nil {
		return
	}
	// Security section
	return
}

func (ga *GitHubActionSummaryImpl) buildInfoSection() (err error) {
	markdown, err := ga.convertContentToMarkdown()
	if err != nil {
		return
	}
	_, err = ga.finalMarkdownFile.WriteString(markdown)
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

func (ga *GitHubActionSummaryImpl) writeUploadedArtifactsToMarkdown() (err error) {
	if err = ga.generateUploadedFilesTree(); err != nil {
		return fmt.Errorf("failed while creating file tree: %w", err)
	}
	if ga.uploadTree.size > 0 {
		if err = ga.writeStringToMarkdown("<details open>\n"); err != nil {
			return
		}
		if err = ga.writeStringToMarkdown("<summary> 📁 Files uploaded to Artifactory by this job </summary>\n\n\n\n"); err != nil {
			return
		}
		if err = ga.writeStringToMarkdown("<pre>\n" + ga.uploadTree.String(true) + "</pre>\n\n"); err != nil {
			return
		}
		if err = ga.writeStringToMarkdown("</details>\n\n"); err != nil {
			return
		}
	}
	return
}

func (ga *GitHubActionSummaryImpl) generateUploadArtifactsTree(contentReader *content.ContentReader) (err error) {
	// Appends the current command upload results to the contentReader file.
	log.Debug("append results to file")
	if err = ga.appendCurrentCommandUploadResults(contentReader); err != nil {
		return fmt.Errorf("failed while appending results: %s", err)
	}
	return
}

// Reads the result file and generates a file tree object.
func (ga *GitHubActionSummaryImpl) generateUploadedFilesTree() (err error) {
	object, err := ga.loadAndMarshalResultsFile()
	if err != nil {
		return
	}
	ga.uploadTree = NewFileTree()
	for _, b := range object.Results {
		ga.uploadTree.AddFile(b.TargetPath, ga.buildUiUrl(b.TargetPath))
	}
	return
}

func (ga *GitHubActionSummaryImpl) buildUiUrl(targetPath string) string {
	template := "%sui/repos/tree/General/%s/?projectKey=%s"
	return fmt.Sprintf(template, ga.platformUrl, targetPath, ga.jfrogProjectKey)
}

func (ga *GitHubActionSummaryImpl) getUploadedArtifactsDataFilePath() string {
	return path.Join(ga.homeDirPath, ga.rawUploadArtifactsFile)
}

// Appends current command results to the data file.
func (ga *GitHubActionSummaryImpl) appendCurrentCommandUploadResults(contentReader *content.ContentReader) error {
	// Read all the current command contentReader files.
	var readContent []UploadResult
	if contentReader != nil {
		for _, file := range contentReader.GetFilesPaths() {
			// Read source file
			sourceBytes, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			// Unmarshal source file content
			var sourceWrapper ResultsWrapper
			err = json.Unmarshal(sourceBytes, &sourceWrapper)
			if err != nil {
				return err
			}
			readContent = append(readContent, sourceWrapper.Results...)
		}
	}
	targetWrapper, err := ga.loadAndMarshalResultsFile()
	if err != nil {
		return err
	}
	// Append source results to target results
	targetWrapper.Results = append(targetWrapper.Results, readContent...)
	// Write target results to target file
	bytes, err := json.Marshal(targetWrapper)
	if err != nil {
		return err
	}
	return os.WriteFile(ga.getUploadedArtifactsDataFilePath(), bytes, 0644)
}

func (ga *GitHubActionSummaryImpl) loadAndMarshalResultsFile() (targetWrapper ResultsWrapper, err error) {
	// Load target file
	targetBytes, err := os.ReadFile(ga.getUploadedArtifactsDataFilePath())
	if err != nil && !os.IsNotExist(err) {
		log.Warn("data file not found ", ga.getUploadedArtifactsDataFilePath())
		return ResultsWrapper{}, err
	}
	if len(targetBytes) == 0 {
		log.Warn("empty data file: ", ga.getUploadedArtifactsDataFilePath())
		return
	}
	// Unmarshal target file content, if it exists
	if err = json.Unmarshal(targetBytes, &targetWrapper); err != nil {
		return
	}
	return
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

func (ga *GitHubActionSummaryImpl) loadDataFileFromSystem() (data []byte, err error) {
	// Read the content of the file
	data, err = fileutils.ReadFile(ga.getDataFileName())
	log.Debug("reading build info data: ", string(data))
	return
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

func initiateGithubSummary() (gh *GitHubActionSummaryImpl, err error) {
	gh, err = newGithubActionSummary()
	if err != nil {
		return
	}
	if err = gh.ensureHomeDirExists(); err != nil {
		return nil, err
	}
	if err = gh.createTempFileIfNeeded(gh.getUploadedArtifactsDataFilePath(), ResultsWrapper{Results: []UploadResult{}}); err != nil {
		return nil, err
	}
	return
}

func newGithubActionSummary() (gh *GitHubActionSummaryImpl, err error) {
	homedir, err := getHomeDirByOs()
	if err != nil {
		return
	}
	gh = &GitHubActionSummaryImpl{
		userMethods:            &GithubSummaryBpImpl{},
		homeDirPath:            homedir,
		rawUploadArtifactsFile: uploadedArtifactsFileName,
		finalMarkdownFile:      nil,
		platformUrl:            utils.AddTrailingSlashIfNeeded(os.Getenv("JF_URL")),
		jfrogProjectKey:        os.Getenv("JFROG_CLI_PROJECT"),
	}
	return
}

func isGitHubActionsRunner() bool {
	return os.Getenv(githubActionsEnv) == "true"
}

func parseBuildTime(timestamp string) string {
	// Parse the timestamp string into a time.Time object
	t, err := time.Parse("2006-01-02T15:04:05.000-0700", timestamp)
	if err != nil {
		return "N/A"
	}
	// Format the time in a more human-readable format and save it in a variable
	return t.Format("Jan 2, 2006 15:04:05")
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
