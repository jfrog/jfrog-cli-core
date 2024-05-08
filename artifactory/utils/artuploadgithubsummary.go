package utils

import (
	"encoding/json"
	"fmt"
	buildinfo "github.com/jfrog/build-info-go/entities"
)

type GithubSummaryRtUploadImpl struct {
	uploadTree        *FileTree // Upload a tree object to generate markdown
	uploadedArtifacts []buildinfo.Artifact
}

type UploadResult struct {
	SourcePath string `json:"sourcePath"`
	TargetPath string `json:"targetPath"`
	RtUrl      string `json:"rtUrl"`
}

type ResultsWrapper struct {
	Results []UploadResult `json:"results"`
}

func NewGithubSummaryRtUploadImpl() *GitHubActionSummaryImpl {
	return &GitHubActionSummaryImpl{userMethods: &GithubSummaryRtUploadImpl{}}
}

func (ga *GithubSummaryRtUploadImpl) convertContentToMarkdown(content []byte) (markdown string, err error) {

	if err = ga.generateUploadedFilesTree(content); err != nil {
		return "", fmt.Errorf("failed while creating file tree: %w", err)
	}
	//if ga.uploadTree.size > 0 {
	//	if err = ga.writeStringToMarkdown("<details open>\n"); err != nil {
	//		return
	//	}
	//	if err = ga.writeStringToMarkdown("<summary> 📁 Files uploaded to Artifactory by this job </summary>\n\n\n\n"); err != nil {
	//		return
	//	}
	//	if err = ga.writeStringToMarkdown("<pre>\n" + ga.uploadTree.String(true) + "</pre>\n\n"); err != nil {
	//		return
	//	}
	//	if err = ga.writeStringToMarkdown("</details>\n\n"); err != nil {
	//		return
	//	}
	//}
	return
}

func (ga *GithubSummaryRtUploadImpl) handleSpecificObject(output interface{}, previousObjectsBytes []byte) (data []byte, err error) {
	artifacts := output.([]buildinfo.Artifact)

	if len(previousObjectsBytes) > 0 {
		err = json.Unmarshal(previousObjectsBytes, &ga.uploadedArtifacts)
		if err != nil {
			return
		}
	} else {
		ga.uploadedArtifacts = make([]buildinfo.Artifact, 0)
	}

	ga.uploadedArtifacts = append(ga.uploadedArtifacts, artifacts...)
	return json.Marshal(ga.uploadedArtifacts)
}

func (ga *GithubSummaryRtUploadImpl) getDataFileName() string {
	return "upload-data-info.json"
}

// Reads the result file and generates a file tree object.
func (ga *GithubSummaryRtUploadImpl) generateUploadedFilesTree(content any) (err error) {

	// Unmarshal the data into an array of build info objects
	//if err = json.Unmarshal(content, &ga.uploadedArtifacts); err != nil {
	//	log.Error("Failed to unmarshal data: ", err)
	//	return
	//}
	//
	//ga.uploadTree = NewFileTree()
	//for _, b := range ga.uploadedArtifacts {
	//	ga.uploadTree.AddFile(b.Path, ga.buildUiUrl(b.TargetPath))
	//}
	return
}
