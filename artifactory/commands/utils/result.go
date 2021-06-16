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

func UnmarshalDeployableArtifacts(filePath string) (*Result, error) {
	modulesMap, err := jsonFileToModulesMap(filePath)
	if err != nil {
		return nil, err
	}
	// Iterate map for : counting seccesses/failures & save artifact's SourcePath, TargetPath and Sha256.
	succeeded, failed := 0, 0
	var artifactsArray []clientutils.FileTransferDetails
	for _, module := range *modulesMap {
		for _, artifact := range module {
			if artifact.DeploySucceeded {
				succeeded++
				artifactsArray = append(artifactsArray, artifact.CreateFileTransferDetails())
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

func jsonFileToModulesMap(filesPath string) (*map[string][]clientutils.DeployableArtifactDetails, error) {
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
