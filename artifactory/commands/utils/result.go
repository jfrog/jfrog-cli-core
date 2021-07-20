package utils

import (
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"io/ioutil"
	"os"
)

const (
	ConfigDeployerPrefix = "deployer"
	GradleConfigRepo     = "repo"
	ConfigServerId       = "serverid"
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
// deployableArtifactsFilePath - path to deployableArtifacts file written by buildinfo project.
// ProjectConfigPath - path to gradle/maven config yaml path.
func UnmarshalDeployableArtifacts(deployableArtifactsFilePath, ProjectConfigPath string) (*Result, error) {
	modulesMap, err := unmarshalDeployableArtifactsJson(deployableArtifactsFilePath)
	if err != nil {
		return nil, err
	}
	url, repo, err := GetDeployerUrlAndRepo(modulesMap, ProjectConfigPath)
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
				artifactsArray = append(artifactsArray, artifact.CreateFileTransferDetails(url, repo))
			} else {
				failed++
			}
		}
	}
	err = clientutils.SaveFileTransferDetailsInFile(deployableArtifactsFilePath, &artifactsArray)
	// Return result
	result := new(Result)
	result.SetSuccessCount(succeeded)
	result.SetFailCount(failed)
	result.SetReader(content.NewContentReader(deployableArtifactsFilePath, "files"))
	return result, nil
}

func GetDeployerUrlAndRepo(modulesMap *map[string][]clientutils.DeployableArtifactDetails, configPath string) (string, string, error) {
	repo := getTargetRepoFromMap(modulesMap)
	vConfig, err := utils.ReadConfigFile(configPath, utils.YAML)
	if err != nil {
		return "", "", err
	}
	// Relevant deploy repository will be written by the buildinfo project in diplyableArtifacts file from gradle extractor v2.24.12.
	// In case of a gradle project with a configuration of 'usePugin=true' its possible that an old buildinfo version is being used.
	// Deploy repository will be read from the configuration file.
	if repo == "" {
		repo = vConfig.GetString(ConfigDeployerPrefix + "." + GradleConfigRepo)
	}
	artDetails, err := config.GetSpecificConfig(vConfig.GetString(ConfigDeployerPrefix+"."+ConfigServerId), true, true)
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
