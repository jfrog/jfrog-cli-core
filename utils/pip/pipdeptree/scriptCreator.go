package main

import (
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

const (
	pipDepTreeVersion             = "3"
	pipDepTreeContentFileName     = "deptreescript.go"
	pipDepTreePythonScript        = "pipdeptree.py"
	pipDepTreeContentRelativePath = "utils/pip"
)

// Run manually  to generates new pipDepTreeContentFileName from pipDepTreePythonScript if there is new one and update pipDepTreeVersion number
func main() {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	// Check if a content file of the latest version already exists
	pipDepTreeContentPath := path.Join(wd, pipDepTreeContentRelativePath, pipDepTreeContentFileName)
	exists, err := fileutils.IsFileExists(pipDepTreeContentPath, false)
	if err != nil {
		panic(err)
	}
	if exists {
		return
	}
	// Read the script content from the .py file
	pyFile, err := ioutil.ReadFile(path.Join(wd, pipDepTreeContentRelativePath, "pipdeptree", pipDepTreePythonScript))
	if err != nil {
		panic(err)
	}
	// Replace all backticks ( ` ) with a single quote ( ' )
	pyFileString := strings.ReplaceAll(string(pyFile), "`", "'")
	// Create .go file with the script content
	// Add it the relevant package
	resourceString := "package piputils\n\n"
	// Add a const string with the script's version
	resourceString += "const pipDepTreeVersion = \"" + pipDepTreeVersion + "\"\n\n"
	// Write the script content a a byte-slice
	resourceString += "var pipDepTreeContent = []byte(`\n" + pyFileString + "`)"

	err = ioutil.WriteFile(pipDepTreeContentPath, []byte(resourceString), os.ModePerm)
	if err != nil {
		panic(err)
	}
}
