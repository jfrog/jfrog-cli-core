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
	"time"
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
	publishedBuildInfo     []*buildInfo.BuildInfo
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
	object, err := gh.loadAndMarshalResultsFile()
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
	targetWrapper, err := gh.loadAndMarshalResultsFile()
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
	return os.WriteFile(gh.getUploadedArtifactsDataFilePath(), bytes, 0644)
}

func (gh *GitHubActionSummary) loadAndMarshalResultsFile() (targetWrapper ResultsWrapper, err error) {
	// Load target file
	targetBytes, err := os.ReadFile(gh.getUploadedArtifactsDataFilePath())
	if err != nil && !os.IsNotExist(err) {
		log.Warn("data file not found ", gh.getUploadedArtifactsDataFilePath())
		return ResultsWrapper{}, err
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

	WriteStringToFile(file, "<p >\n  <h1> \n    <picture><img src=\"https://github.com/jfrog/jfrog-cli-core/assets/23456142/d2df3c49-30a6-4eb6-be66-42014b17d1fb\" style=\"margin: 0 0 -15px 0\"width=\"100px\"></picture>  CLI Github Action Summary \n     </h1> \n</p>  \n\n")

	if gh.uploadTree.size > 0 {
		WriteStringToFile(file, "<details open>\n")
		WriteStringToFile(file, "<summary> 📁 Files uploaded to Artifactory by this workflow </summary>\n\n")
		WriteStringToFile(file, "```\n"+gh.uploadTree.String()+"\n```\n")
		WriteStringToFile(file, "</details>\n\n")
	}

	if err = gh.loadBuildInfoData(); err != nil {
		return
	}

	if len(gh.publishedBuildInfo) > 0 {
		WriteStringToFile(file, "<details open>\n\n")
		WriteStringToFile(file, "<summary> 📦 Build Info published to Artifactory by this workflow </summary>\n\n")
		WriteStringToFile(file, gh.buildInfoTable())
		WriteStringToFile(file, "\n</details>\n")
	}

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
	// Generate a string that represents a Markdown table
	var tableBuilder strings.Builder
	tableBuilder.WriteString("| Build | Timestamp | \n")
	tableBuilder.WriteString("|---------|------------| \n")
	for _, build := range gh.publishedBuildInfo {
		buildTime := parseBuildTime(build.Started)
		tableBuilder.WriteString(fmt.Sprintf("| [%s](%s) | %s |\n", build.Name+build.Number, build.BuildUrl, buildTime))
	}
	return tableBuilder.String()
}

func parseBuildTime(timestamp string) string {
	// Parse the timestamp string into a time.Time object
	t, err := time.Parse("2006-01-02T15:04:05.000-0700", timestamp)
	if err != nil {
		return "N/A"
	}
	// Format the time in a more human-readable format and save it in a variable
	return t.Format("Mon Jan _2 15:04:05 2006")
}

func (gh *GitHubActionSummary) loadBuildInfoData() (err error) {
	log.Info("building build info table...")
	// Read the content of the file
	data, err := fileutils.ReadFile(gh.getPublishedBuildInfoDataFilePath())
	log.Debug("reading build info data: ", string(data))
	if err != nil {
		log.Error("Failed to read file: ", err)
		return
	}
	// Unmarshal the data into an array of build info objects
	err = json.Unmarshal(data, &gh.publishedBuildInfo)
	if err != nil {
		log.Error("Failed to unmarshal data: ", err)
		return
	}
	return
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
	if err = gh.createTempFileIfNeeded(gh.getPublishedBuildInfoDataFilePath(), []*buildInfo.BuildInfo{}); err != nil {
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
		publishedBuildInfo:     make([]*buildInfo.BuildInfo, 0),
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
		// TODO remove this,used for developing
		return filepath.Join("/Users/eyalde/IdeaProjects/githubRunner/_work", ".jfrog", "jfrog-github-summary")
	}
}

func GitHubJobSummariesCollectBuildInfoData(build *buildInfo.BuildInfo) (err error) {
	if os.Getenv("GITHUB_ACTIONS") != "true" {
		return nil
	}
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
