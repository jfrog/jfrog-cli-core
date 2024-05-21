package commandssummaries

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/commandsummary"
)

type UploadSummary struct {
	uploadTree        *utils.FileTree
	uploadedArtifacts ResultsWrapper
	PlatformUrl       string
	JfrogProjectKey   string
}

type UploadResult struct {
	SourcePath string `json:"sourcePath"`
	TargetPath string `json:"targetPath"`
	RtUrl      string `json:"rtUrl"`
}

type ResultsWrapper struct {
	Results []UploadResult `json:"results"`
}

func NewUploadSummary(platformUrl, projectKey string) *UploadSummary {
	return &UploadSummary{
		PlatformUrl:     platformUrl,
		JfrogProjectKey: projectKey,
	}
}

func (us *UploadSummary) GenerateMarkdownFromFiles(dataFilePaths []string) (markdown string, err error) {
	if err = us.loadResults(dataFilePaths); err != nil {
		return
	}
	// Wrap the markdown in a <pre> tags to preserve spaces
	markdown = fmt.Sprintf("\n<pre>\n\n\n" + us.generateFileTreeMarkdown() + "</pre>\n\n")
	return
}

// Loads all the recorded results from the given file paths.
func (us *UploadSummary) loadResults(filePaths []string) error {
	us.uploadedArtifacts = ResultsWrapper{}
	for _, path := range filePaths {
		var uploadResult ResultsWrapper
		if err := commandsummary.UnmarshalFromFilePath(path, &uploadResult); err != nil {
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
	template := "%sui/repos/tree/General/%s/?projectKey=%s"
	return fmt.Sprintf(template, us.PlatformUrl, targetPath, us.JfrogProjectKey)
}
