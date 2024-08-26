package commandsummary

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
)

type UploadSummary struct {
	CommandSummary
	uploadTree        *utils.FileTree
	uploadedArtifacts ResultsWrapper
}

func (us *UploadSummary) GetSummaryTitle() string {
	return "📁 Files uploaded to Artifactory by this workflow"
}

type UploadResult struct {
	SourcePath string `json:"sourcePath"`
	TargetPath string `json:"targetPath"`
	RtUrl      string `json:"rtUrl"`
}

type ResultsWrapper struct {
	Results []UploadResult `json:"results"`
}

func NewUploadSummary() (*CommandSummary, error) {
	return New(&UploadSummary{}, "upload")
}

func (us *UploadSummary) GenerateMarkdownFromFiles(dataFilePaths []string) (markdown string, err error) {
	if err = us.loadResults(dataFilePaths); err != nil {
		return
	}
	// Wrap the Markdown in a <pre> tags to preserve spaces
	markdown = fmt.Sprintf("\n<pre>\n\n\n%s</pre>\n\n", us.generateFileTreeMarkdown())
	return
}

// Loads all the recorded results from the given file paths.
func (us *UploadSummary) loadResults(filePaths []string) error {
	us.uploadedArtifacts = ResultsWrapper{}
	for _, path := range filePaths {
		var uploadResult ResultsWrapper
		if err := UnmarshalFromFilePath(path, &uploadResult); err != nil {
			return err
		}
		us.uploadedArtifacts.Results = append(us.uploadedArtifacts.Results, uploadResult.Results...)
	}
	return nil
}

func (us *UploadSummary) generateFileTreeMarkdown() string {
	us.uploadTree = utils.NewFileTree()
	for _, uploadResult := range us.uploadedArtifacts.Results {
		us.uploadTree.AddFile(uploadResult.TargetPath, us.buildUiUrl(uploadResult.TargetPath))
	}
	return us.uploadTree.String()
}

func (us *UploadSummary) buildUiUrl(targetPath string) string {
	// Only build URL if extended summary is enabled
	if StaticMarkdownConfig.IsExtendedSummary() {
		return GenerateArtifactUrl(targetPath)
	}
	return ""
}
