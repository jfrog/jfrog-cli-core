package utils

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
)

const (
	configDeployerPrefix = "deployer"
	gradleConfigRepo     = "repo"
	configServerId       = "serverid"
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

// UnmarshalDeployableArtifacts reads and parses the deployed artifacts details from the provided file.
// The details were written by Build-info project while deploying artifacts to maven and gradle repositories.
// deployableArtifactsFilePath - path to deployableArtifacts file written by Build-info project.
// projectConfigPath - path to gradle/maven config yaml path.
// lateDeploy - boolean indicates if the artifacts was expected to be deployed.
func UnmarshalDeployableArtifacts(deployableArtifactsFilePath, projectConfigPath string, lateDeploy bool) (*Result, error) {
	modulesMap, err := unmarshalDeployableArtifactsJson(deployableArtifactsFilePath)
	if err != nil {
		return nil, err
	}
	url, repo, err := getDeployerUrlAndRepo(modulesMap, projectConfigPath)
	if err != nil {
		return nil, err
	}
	// Iterate over the modules map, counting successes/failures & save artifact's SourcePath, TargetPath, and Sha256.
	succeeded, failed := 0, 0
	var artifactsArray []clientutils.FileTransferDetails
	for _, module := range *modulesMap {
		for _, artifact := range module {
			if lateDeploy || artifact.DeploySucceeded {
				artifactDetails, err := artifact.CreateFileTransferDetails(url, repo)
				if err != nil {
					return nil, err
				}
				artifactsArray = append(artifactsArray, artifactDetails)
				succeeded++
			} else {
				failed++
			}
		}
	}
	err = clientutils.SaveFileTransferDetailsInFile(deployableArtifactsFilePath, &artifactsArray)
	if err != nil {
		return nil, err
	}
	// Return result
	result := new(Result)
	result.SetSuccessCount(succeeded)
	result.SetFailCount(failed)
	result.SetReader(content.NewContentReader(deployableArtifactsFilePath, "files"))
	return result, nil
}

// getDeployerUrlAndRepo returns the deployer url and the target repository for maven and gradle.
// Url is being read from the project's local configuration file.
// Repository is being read from the modulesMap.
// modulesMap - map of the DeployableArtifactDetails.
// configPath -  path to the project's local configuration file.
func getDeployerUrlAndRepo(modulesMap *map[string][]clientutils.DeployableArtifactDetails, configPath string) (string, string, error) {
	repo := getTargetRepoFromMap(modulesMap)
	vConfig, err := utils.ReadConfigFile(configPath, utils.YAML)
	if err != nil {
		return "", "", err
	}
	// The relevant deployment repository will be written by the build-info project to the deployableArtifacts file starting from version 2.24.12 of build-info-extractor-gradle.
	// In case of a gradle project with a configuration of 'usePlugin=true' it's possible that an old build-info-extractor-gradle version is being used.
	// In this case, the value of "repo" will be empty, and the deployment repository will be therefore read from the local project configuration file.
	if repo == "" {
		repo = vConfig.GetString(configDeployerPrefix + "." + gradleConfigRepo)
	}
	artDetails, err := config.GetSpecificConfig(vConfig.GetString(configDeployerPrefix+"."+configServerId), true, true)
	if err != nil {
		return "", "", err
	}
	url := artDetails.ArtifactoryUrl
	return url, repo, nil
}

func getTargetRepoFromMap(modulesMap *map[string][]clientutils.DeployableArtifactDetails) string {
	for _, module := range *modulesMap {
		for _, artifact := range module {
			return artifact.TargetRepository
		}
	}
	return ""
}

func unmarshalDeployableArtifactsJson(filesPath string) (*map[string][]clientutils.DeployableArtifactDetails, error) {
	// Open the file
	jsonFile, err := os.Open(filesPath)
	defer func() {
		e := jsonFile.Close()
		if err == nil {
			err = errorutils.CheckError(e)
		}
	}()
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	// Read and convert json file to a modules map
	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	var modulesMap map[string][]clientutils.DeployableArtifactDetails
	err = json.Unmarshal(byteValue, &modulesMap)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	return &modulesMap, nil
}
