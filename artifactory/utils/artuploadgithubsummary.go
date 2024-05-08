package utils

import (
	"encoding/json"
	"fmt"
	"strings"
)

type GithubSummaryRtUploadImpl struct {
	uploadTree        *FileTree // Upload a tree object to generate markdown
	uploadedArtifacts ResultsWrapper
	platformUrl       string
	jfrogProjectKey   string
}

type UploadResult struct {
	SourcePath string `json:"sourcePath"`
	TargetPath string `json:"targetPath"`
	RtUrl      string `json:"rtUrl"`
}

type ResultsWrapper struct {
	Results []UploadResult `json:"results"`
}

func (ga *GithubSummaryRtUploadImpl) renderContentToMarkdown(content []byte) (markdown string, err error) {

	if err = ga.generateUploadedFilesTree(content); err != nil {
		return "", fmt.Errorf("failed while creating file tree: %w", err)
	}
	var markdownBuilder strings.Builder
	if ga.uploadTree.size > 0 {
		if _, err = markdownBuilder.WriteString("<details open>\n"); err != nil {
			return
		}
		if _, err = markdownBuilder.WriteString("<summary> 📁 Files uploaded to Artifactory by this job </summary>\n\n\n\n"); err != nil {
			return
		}
		if _, err = markdownBuilder.WriteString("<pre>\n" + ga.uploadTree.String(true) + "</pre>\n\n"); err != nil {
			return
		}
		if _, err = markdownBuilder.WriteString("</details>\n\n"); err != nil {
			return
		}
	}
	return markdownBuilder.String(), nil
}

func (ga *GithubSummaryRtUploadImpl) appendResultObject(output interface{}, previousObjectsBytes []byte) (data []byte, err error) {
	currentResults, ok := output.([]byte)
	if !ok {
		return nil, fmt.Errorf("failed to convert output to []byte")
	}
	currentUpload := ResultsWrapper{}
	if err = json.Unmarshal(currentResults, &currentUpload); err != nil {
		return
	}

	if len(previousObjectsBytes) > 0 {
		err = json.Unmarshal(previousObjectsBytes, &ga.uploadedArtifacts)
		if err != nil {
			return
		}
	} else {
		ga.uploadedArtifacts = ResultsWrapper{}
	}

	ga.uploadedArtifacts.Results = append(ga.uploadedArtifacts.Results, currentUpload.Results...)
	return json.Marshal(ga.uploadedArtifacts)
}

// Reads the result file and generates a file tree object.
func (ga *GithubSummaryRtUploadImpl) generateUploadedFilesTree(content any) (err error) {
	currentResults, ok := content.([]byte)
	if !ok {
		return fmt.Errorf("failed to convert content to []byte")
	}
	currentUpload := ResultsWrapper{}
	if err = json.Unmarshal(currentResults, &currentUpload); err != nil {
		return
	}
	ga.uploadTree = NewFileTree()
	for _, b := range currentUpload.Results {
		ga.uploadTree.AddFile(b.TargetPath, ga.buildUiUrl(b.TargetPath))
	}
	return
}

func (ga *GithubSummaryRtUploadImpl) buildUiUrl(targetPath string) string {
	template := "%sui/repos/tree/General/%s/?projectKey=%s"
	return fmt.Sprintf(template, ga.platformUrl, targetPath, ga.jfrogProjectKey)
}
