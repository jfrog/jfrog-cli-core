package jas

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"math/rand"
	"os"
	"time"
)

const analyzerManagerFilePath = "/Users/ort/workspace/src/jfrog.com/analyzerManager" // todo add real path

func isTechEligibleForJas(tech coreutils.Technology, eligibleTechnologies []coreutils.Technology) bool {
	for _, eligibleTech := range eligibleTechnologies {
		if tech == eligibleTech {
			return true
		}
	}
	return false
}

func isAnalyzerManagerExecutableExist() error {
	if _, err := os.Stat(analyzerManagerFilePath); err != nil {
		return err
	}
	return nil
}

func generateRandomFileName() string {
	rand.Seed(time.Now().UnixNano())
	const nameLength = 10
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	fileName := make([]rune, nameLength)
	for i := range fileName {
		fileName[i] = letters[rand.Intn(len(letters))]
	}
	return string(fileName)
}
