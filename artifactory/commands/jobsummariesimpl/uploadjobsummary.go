package jobsummariesimpl

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/jobsummaries"
	"strings"
)

type GithubSummaryRtUploadImpl struct {
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

func (ga *GithubSummaryRtUploadImpl) CreateSummaryMarkdown(content any, section jobsummaries.MarkdownSection) (err error) {
	return jobsummaries.CreateSummaryMarkdownBaseImpl(content, section, ga.appendResultObject, ga.renderContentToMarkdown)
}

func (ga *GithubSummaryRtUploadImpl) renderContentToMarkdown(content []byte) (markdown string, err error) {
	if err = ga.generateUploadedFilesTree(content); err != nil {
		return "", fmt.Errorf("failed while creating file tree: %w", err)
	}
	var markdownBuilder strings.Builder
	if ga.uploadTree.String() != "" {
		if _, err = markdownBuilder.WriteString("\n<pre>\n" + ga.uploadTree.String() + "</pre>\n\n"); err != nil {
			return
		}
	}
	return markdownBuilder.String(), nil
}

func (ga *GithubSummaryRtUploadImpl) appendResultObject(currentResult interface{}, previousResults []byte) (data []byte, err error) {
	currentResults, ok := currentResult.([]byte)
	if !ok {
		return nil, fmt.Errorf("failed to convert currentResult to []byte")
	}
	currentUpload := ResultsWrapper{}
	if err = json.Unmarshal(currentResults, &currentUpload); err != nil {
		return
	}

	if len(previousResults) > 0 {
		err = json.Unmarshal(previousResults, &ga.uploadedArtifacts)
		if err != nil {
			return
		}
	} else {
		ga.uploadedArtifacts = ResultsWrapper{}
	}

	ga.uploadedArtifacts.Results = append(ga.uploadedArtifacts.Results, currentUpload.Results...)
	return json.Marshal(ga.uploadedArtifacts)
}

func (ga *GithubSummaryRtUploadImpl) generateUploadedFilesTree(content any) (err error) {
	rawInput, ok := content.([]byte)
	if !ok {
		return fmt.Errorf("failed to convert content to []byte")
	}
	uploadedFileResults := ResultsWrapper{}
	if err = json.Unmarshal(rawInput, &uploadedFileResults); err != nil {
		return
	}

	ga.uploadTree = utils.NewFileTree()
	for _, b := range uploadedFileResults.Results {
		ga.uploadTree.AddFile(b.TargetPath, ga.buildUiUrl(b.TargetPath))
	}
	return
}

func (ga *GithubSummaryRtUploadImpl) buildUiUrl(targetPath string) string {
	template := "%sui/repos/tree/General/%s/?projectKey=%s"
	return fmt.Sprintf(template, ga.PlatformUrl, targetPath, ga.JfrogProjectKey)
}
