package commandsummary

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
)

type UploadSummary struct {
	CommandSummary
	uploadTree        *utils.FileTree
	uploadedArtifacts ResultsWrapper
	platformUrl       string
	majorVersion      int
	extendedSummary   bool
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

func NewUploadSummary(platformUrl string, majorVersion int) (*CommandSummary, error) {
	return New(&UploadSummary{
		platformUrl:  platformUrl,
		majorVersion: majorVersion,
	}, "upload")
}

func (us *UploadSummary) GenerateMarkdownFromFiles(dataFilePaths []string, extendedSummary bool) (markdown string, err error) {
	us.extendedSummary = extendedSummary
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
	if !us.extendedSummary {
		// When the summary is not extended, the UI URL is not generated.
		// When passing empty string to the upload tree, it won't be displayed as a link.
		return ""
	}
	return GenerateArtifactUrl(us.platformUrl, targetPath, us.majorVersion)
}
