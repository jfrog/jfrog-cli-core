package jas

import (
	"math/rand"
	"os"
	"time"
)

const AnalyzerManagerFilePath = "analyzerManager" // todo add real path

func IsTechEligibleForJas(tech string, eligibleTechnologies []string) bool {
	for _, eligibleTech := range eligibleTechnologies {
		if tech == eligibleTech {
			return true
		}
	}
	return false
}

func IsAnalyzerManagerExecutableExist() error {
	if _, err := os.Stat(AnalyzerManagerFilePath); err != nil {
		return err
	}
	return nil
}

func GenerateRandomFileName() string {
	rand.Seed(time.Now().UnixNano())
	const nameLength = 10
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	fileName := make([]rune, nameLength)
	for i := range fileName {
		fileName[i] = letters[rand.Intn(len(letters))]
	}
	return string(fileName)
}

func GetScanRootFolder() string { //todo
	return ""
}

func GetXrayVulnerabilities() []string { //todo
	return []string{}
}
