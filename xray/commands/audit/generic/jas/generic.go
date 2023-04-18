package jas

import (
	"math/rand"
	"time"
)

const analyzerManagerFilePath = "/Users/ort/Documents/am_eco/analyzerManager" // todo add real path

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
