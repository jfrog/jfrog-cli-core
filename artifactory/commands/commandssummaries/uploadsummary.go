package commandssummaries

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/commandsummary"
)

type UploadSummary struct {
	uploadTree        *utils.FileTree // Upload a tree object to generate markdown
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

func (ga *UploadSummary) CreateMarkdown(data any) (err error) {
	return commandsummary.CreateMarkdown(data, "upload", ga.renderContentToMarkdown)
}

func (ga *UploadSummary) renderContentToMarkdown(filePaths []string) (markdown string, err error) {
	if err = ga.loadResults(filePaths); err != nil {
		return
	}
	// Builds the file tree from the loaded results
	ga.generateFileTree()
	// Wrap the file tree in a <pre> tag to preserve spaces
	markdown = fmt.Sprintf("\n<pre>\n" + ga.uploadTree.String() + "</pre>\n\n")
	return
}

// Loads all the recorded results from the given file paths.
func (ga *UploadSummary) loadResults(filePaths []string) error {
	ga.uploadedArtifacts = ResultsWrapper{}
	for _, path := range filePaths {
		var uploadResult ResultsWrapper
		if err := commandsummary.UnmarshalFromFilePath(path, &uploadResult); err != nil {
			return err
		}
		ga.uploadedArtifacts.Results = append(ga.uploadedArtifacts.Results, uploadResult.Results...)
	}
	return nil
}

func (ga *UploadSummary) generateFileTree() {
	ga.uploadTree = utils.NewFileTree()
	for _, b := range ga.uploadedArtifacts.Results {
		ga.uploadTree.AddFile(b.TargetPath, ga.buildUiUrl(b.TargetPath))
	}
}

func (ga *UploadSummary) buildUiUrl(targetPath string) string {
	template := "%sui/repos/tree/General/%s/?projectKey=%s"
	return fmt.Sprintf(template, ga.PlatformUrl, targetPath, ga.JfrogProjectKey)
}
