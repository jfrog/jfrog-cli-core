package utils

import (
	"encoding/json"
	"fmt"
	buildInfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type UploadResult struct {
	SourcePath string `json:"sourcePath"`
	TargetPath string `json:"targetPath"`
	RtUrl      string `json:"rtUrl"`
}

type ResultsWrapper struct {
	Results []UploadResult `json:"results"`
}

type GitHubActionSummary struct {
	homeDirPath            string    // Directory path for the GitHubActionSummary data
	rawUploadArtifactsFile string    // File which contains all the results of the commands
	rawBuildInfoFile       string    // File containing build info results
	uploadTree             *FileTree // Upload a tree object to generate markdown
}

const (
	githubActionsEnv          = "GITHUB_ACTIONS"
	buildInfoFileName         = "build-info-data.json"
	uploadedArtifactsFileName = "data.json"
)

// GenerateGitHubActionSummary TODO this isn't clear why you should pass a content reader,maybe this can be refactoed.
func GenerateGitHubActionSummary(contentReader *content.ContentReader) (err error) {
	if os.Getenv(githubActionsEnv) != "true" {
		return
	}
	// Initiate the GitHubActionSummary, will check for previous runs and aggregate results if needed.
	gh, err := createNewGithubSummary()
	if err != nil {
		return fmt.Errorf("failed while initiating Github job summaries: %w", err)
	}

	if contentReader != nil {
		err = gh.generateUploadArtifactsTree(contentReader)
		if err != nil {
			return err
		}
	}

	// TODO implement scan results

	// Generate the whole markdown
	log.Debug("generating markdown")
	return gh.generateMarkdown()
}

func (gh *GitHubActionSummary) generateUploadArtifactsTree(contentReader *content.ContentReader) (err error) {
	// Appends the current command upload results to the contentReader file.
	log.Debug("append results to file")
	if err = gh.appendCurrentCommandUploadResults(contentReader); err != nil {
		return fmt.Errorf("failed while appending results: %s", err)
	}
	return
}

// Reads the result file and generates a file tree object.
func (gh *GitHubActionSummary) generateUploadedFilesTree() (err error) {
	object, _, err := gh.loadAndMarshalResultsFile()

	if err != nil {
		return
	}
	gh.uploadTree = NewFileTree()
	for _, b := range object.Results {
		gh.uploadTree.AddFile(b.TargetPath)
	}
	return
}

func (gh *GitHubActionSummary) getUploadedArtifactsDataFilePath() string {
	return path.Join(gh.homeDirPath, gh.rawUploadArtifactsFile)
}

func (gh *GitHubActionSummary) getPublishedBuildInfoDataFilePath() string {
	return path.Join(gh.homeDirPath, gh.rawBuildInfoFile)
}

// Appends current command results to the data file.
func (gh *GitHubActionSummary) appendCurrentCommandUploadResults(contentReader *content.ContentReader) error {
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
	targetWrapper, targetBytes, err := gh.loadAndMarshalResultsFile()
	// Append source results to target results
	targetWrapper.Results = append(targetWrapper.Results, readContent...)
	// Marshal target results
	targetBytes, err = json.MarshalIndent(targetWrapper, "", "  ")
	if err != nil {
		return err
	}
	// Write target results to target file
	return os.WriteFile(gh.getUploadedArtifactsDataFilePath(), targetBytes, 0644)
}

func (gh *GitHubActionSummary) loadAndMarshalResultsFile() (targetWrapper ResultsWrapper, targetBytes []byte, err error) {
	// Load target file
	targetBytes, err = os.ReadFile(gh.getUploadedArtifactsDataFilePath())
	if err != nil && !os.IsNotExist(err) {
		log.Warn("data file not found ", gh.getUploadedArtifactsDataFilePath())
		return ResultsWrapper{}, nil, err
	}
	if len(targetBytes) == 0 {
		log.Warn("empty data file: ", gh.getUploadedArtifactsDataFilePath())
		return
	}
	// Unmarshal target file content, if it exists
	if err = json.Unmarshal(targetBytes, &targetWrapper); err != nil {
		return
	}
	return
}

func (gh *GitHubActionSummary) generateMarkdown() (err error) {
	// Generate an upload tree from file
	log.Debug("generate uploaded files tree")
	if err = gh.generateUploadedFilesTree(); err != nil {
		return fmt.Errorf("failed while creating file tree: %w", err)
	}

	tempMarkdownPath := path.Join(gh.homeDirPath, "summary.md")
	// Remove the file if it exists
	if err = os.Remove(tempMarkdownPath); err != nil {
		log.Debug("failed to remove old markdown file: ", err)
	}
	file, err := os.OpenFile(tempMarkdownPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer func() {
		err = file.Close()
	}()
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	WriteStringToFile(file, "# 🐸 JFrog CLI Github Action Summary 🐸\n")
	WriteStringToFile(file, "## 📁 Uploaded artifacts:\n")
	WriteStringToFile(file, "```\n"+gh.uploadTree.String()+"```\n")
	WriteStringToFile(file, "## 📦 Published Build info \n ")
	WriteStringToFile(file, gh.buildInfoTable())
	return
}

func (gh *GitHubActionSummary) createTempFileIfNeeded(filePath string, content any) (err error) {
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

func (gh *GitHubActionSummary) ensureHomeDirExists() error {
	if _, err := os.Stat(gh.homeDirPath); os.IsNotExist(err) {
		err = os.MkdirAll(gh.homeDirPath, 0755)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func (gh *GitHubActionSummary) buildInfoTable() string {
	log.Info("building build info table...")
	// Read the content of the file
	data, err := fileutils.ReadFile(gh.getPublishedBuildInfoDataFilePath())
	if err != nil {
		log.Error("Failed to read file: ", err)
		return ""
	}

	// Unmarshal the data into an array of build info objects
	var builds []*buildInfo.BuildInfo
	err = json.Unmarshal(data, &builds)
	if err != nil {
		log.Error("Failed to unmarshal data: ", err)
		return ""
	}

	// Generate a string that represents a Markdown table
	var tableBuilder strings.Builder
	tableBuilder.WriteString("| Name | Number | Agent Name | Agent Version | Build Agent Name | Build Agent Version | Started | Artifactory Principal |\n")
	tableBuilder.WriteString("|------|--------|------------|---------------|------------------|---------------------|---------|----------------------|\n")
	for _, build := range builds {
		tableBuilder.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s | %s |\n", build.Name, build.Number, build.Agent.Name, build.Agent.Version, build.BuildAgent.Name, build.BuildAgent.Version, build.Started, build.Principal))
	}
	log.Info("build info table: ", tableBuilder.String())
	return tableBuilder.String()
}

// Initializes a new GitHubActionSummary
func createNewGithubSummary() (gh *GitHubActionSummary, err error) {
	gh = newGithubActionSummary()
	if err = gh.ensureHomeDirExists(); err != nil {
		return nil, err
	}
	if err = gh.createTempFileIfNeeded(gh.getUploadedArtifactsDataFilePath(), ResultsWrapper{Results: []UploadResult{}}); err != nil {
		return nil, err
	}
	if err = gh.createTempFileIfNeeded(gh.getPublishedBuildInfoDataFilePath(), ResultsWrapper{Results: []UploadResult{}}); err != nil {
		return nil, err
	}
	return
}

func newGithubActionSummary() (gh *GitHubActionSummary) {
	homedir := GetHomeDirByOs()
	gh = &GitHubActionSummary{
		homeDirPath:            homedir,
		rawUploadArtifactsFile: uploadedArtifactsFileName,
		rawBuildInfoFile:       buildInfoFileName,
	}
	return gh
}

func WriteStringToFile(file *os.File, str string) {
	_, err := file.WriteString(str)
	if err != nil {
		log.Error(fmt.Errorf("failed to write string to file: %w", err))
	}
}

func GetHomeDirByOs() string {
	switch osString := os.Getenv("RUNNER_OS"); osString {
	case "Windows":
		return filepath.Join(os.Getenv("USERPROFILE"), ".jfrog", "jfrog-github-summary")
	case "Linux", "macOS":
		return filepath.Join(os.Getenv("HOME"), ".jfrog", "jfrog-github-summary")
	default:
		return ""
	}
}

func WriteBuildInfoData(build *buildInfo.BuildInfo) (err error) {
	filePath := path.Join(GetHomeDirByOs(), buildInfoFileName)
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0644)
	defer func() {
		err = file.Close()
	}()
	if err != nil {
		return
	}
	// Read the existing data
	data, err := fileutils.ReadFile(filePath)
	if err != nil {
		return err
	}
	// Unmarshal the data into an array of build info objects
	var builds []*buildInfo.BuildInfo
	if len(data) > 0 {
		err = json.Unmarshal(data, &builds)
		if err != nil {
			return err
		}
	}
	// Append the new build info object to the array
	builds = append(builds, build)
	// Marshal the updated array back into JSON
	updatedData, err := json.Marshal(builds)
	if err != nil {
		return err
	}
	_, err = file.Write(updatedData)
	if err != nil {
		return err
	}
	return
}
