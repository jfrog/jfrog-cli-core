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
