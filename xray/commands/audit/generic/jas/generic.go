package jas

import (
	"errors"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"os"
	"path/filepath"
)

var (
	analyzerManagerFilePath  = "analayzerManager/analyzerManager"
	analyzerManagerLogFolder = ""
)

const (
	analyzerManagerDirName   = "analyzerManagerLogs"
	jfUserEnvVariable        = "JF_USER"
	jfPasswordEnvVariable    = "JF_PASS"
	jfTokenEnvVariable       = "JF_TOKEN"
	jfPlatformUrlEnvVariable = "JF_PLATFORM_URL"
	logDirEnvVariable        = "AM_LOG_DIRECTORY"
)

func createAnalyzerManagerLogDir() error {
	logDir, err := coreutils.CreateDirInJfrogHome(filepath.Join(coreutils.JfrogLogsDirName, analyzerManagerDirName))
	if err != nil {
		return err
	}
	analyzerManagerLogFolder = logDir
	return nil
}

func getAnalyzerManagerAbsolutePath() (string, error) {
	jfrogDir, err := config.GetJfrogDependenciesPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(jfrogDir, analyzerManagerFilePath), nil
}

func removeDuplicateValues(stringSlice []string) []string {
	keys := make(map[string]bool)
	finalSlice := []string{}
	for _, entry := range stringSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			finalSlice = append(finalSlice, entry)
		}
	}
	return finalSlice
}

func setAnalyzerManagerEnvVariables(serverDetails *config.ServerDetails) error {
	if serverDetails == nil {
		return errors.New("cant get xray server details")
	}
	err := os.Setenv(jfUserEnvVariable, serverDetails.User)
	if err != nil {
		return err
	}
	err = os.Setenv(jfPasswordEnvVariable, serverDetails.Password)
	if err != nil {
		return err
	}
	err = os.Setenv(jfPlatformUrlEnvVariable, serverDetails.Url)
	if err != nil {
		return err
	}
	err = os.Setenv(jfTokenEnvVariable, serverDetails.AccessToken)
	if err != nil {
		return err
	}
	err = os.Setenv(logDirEnvVariable, analyzerManagerLogFolder)
	return err
}
