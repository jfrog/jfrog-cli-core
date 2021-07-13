package utils

import (
	"encoding/json"
	"io/ioutil"
	"os"

	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
)

type Result struct {
	successCount int
	failCount    int
	reader       *content.ContentReader
}

func (r *Result) SuccessCount() int {
	return r.successCount
}

func (r *Result) FailCount() int {
	return r.failCount
}

func (r *Result) Reader() *content.ContentReader {
	return r.reader
}

func (r *Result) SetSuccessCount(successCount int) {
	r.successCount = successCount
}

func (r *Result) SetFailCount(failCount int) {
	r.failCount = failCount
}

func (r *Result) SetReader(reader *content.ContentReader) {
	r.reader = reader
}

// UnmarshalDeployableArtifacts Reads and parses the deployed artifacts details from the provided file.
// The details were written by Buildinfo project while deploying artifacts to maven and gradle repositories.
func UnmarshalDeployableArtifacts(filePath, rtUrl string) (*Result, error) {
	modulesMap, err := unmarshalDeployableArtifactsJson(filePath)
	if err != nil {
		return nil, err
	}
	// Iterate over the modules map , counting seccesses/failures & save artifact's SourcePath, TargetPath and Sha256.
	succeeded, failed := 0, 0
	var artifactsArray []clientutils.FileTransferDetails
	for _, module := range *modulesMap {
		for _, artifact := range module {
			if artifact.DeploySucceeded {
				succeeded++
				artifactsArray = append(artifactsArray, artifact.CreateFileTransferDetails(rtUrl))
			} else {
				failed++
			}
		}
	}
	err = clientutils.SaveFileTransferDetailsInFile(filePath, &artifactsArray)
	// Return result
	result := new(Result)
	result.SetSuccessCount(succeeded)
	result.SetFailCount(failed)
	result.SetReader(content.NewContentReader(filePath, "files"))
	return result, nil
}

func unmarshalDeployableArtifactsJson(filesPath string) (*map[string][]clientutils.DeployableArtifactDetails, error) {
	// Open the file
	jsonFile, err := os.Open(filesPath)
	defer jsonFile.Close()
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	// Read and convert json file to a modules map
	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	var modulesMap map[string][]clientutils.DeployableArtifactDetails
	err = json.Unmarshal([]byte(byteValue), &modulesMap)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	return &modulesMap, nil
}
