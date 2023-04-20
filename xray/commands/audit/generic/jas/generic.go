package jas

import (
	"crypto/rand"
	"errors"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"math/big"
	"os"
	"path/filepath"
)

const (
	analyzerManagerFilePath = "~/.jfrog/dependencies/analayzerManager/analyzerManager"
	jfUserEnvVariable       = "JF_USER"
	jfPasswordEnvVariable   = "JF_PASS"
	jfPlatformUrl           = "JF_PLATFORM_URL"
)

func getAnalyzerManagerAbsolutePath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, analyzerManagerFilePath)
}

func generateRandomFileName() (string, error) {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"
	result := make([]byte, 10)
	for i := 0; i < 10; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", err
		}
		result[i] = letters[num.Int64()]
	}

	return string(result), nil
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
	err = os.Setenv(jfPlatformUrl, serverDetails.Url)
	if err != nil {
		return err
	}
	return nil
}
