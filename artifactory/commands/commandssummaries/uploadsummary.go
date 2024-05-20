package commandssummaries

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/commandsummary"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"os"
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

func NewUploadSummary() *UploadSummary {
	return &UploadSummary{
		PlatformUrl:     clientUtils.AddTrailingSlashIfNeeded(os.Getenv(commandsummary.PlatformUrlEnv)),
		JfrogProjectKey: os.Getenv(coreutils.Project),
	}
}

func (ga *UploadSummary) GenerateMarkdownFromFiles(dataFilePaths []string) (finalMarkdown string, err error) {
	if err = ga.loadResults(dataFilePaths); err != nil {
		return
	}
	// Wrap the markdown in a <pre> tags to preserve spaces
	finalMarkdown = fmt.Sprintf("\n<pre>\n\n\n" + ga.generateFileTreeMarkdown() + "</pre>\n\n")
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

func (ga *UploadSummary) generateFileTreeMarkdown() string {
	ga.uploadTree = utils.NewFileTree()
	for _, b := range ga.uploadedArtifacts.Results {
		ga.uploadTree.AddFile(b.TargetPath, ga.buildUiUrl(b.TargetPath))
	}
	return ga.uploadTree.String()
}

func (ga *UploadSummary) buildUiUrl(targetPath string) string {
	template := "%sui/repos/tree/General/%s/?projectKey=%s"
	return fmt.Sprintf(template, ga.PlatformUrl, targetPath, ga.JfrogProjectKey)
}
