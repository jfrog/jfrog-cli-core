package tests

import (
	"github.com/jfrog/jfrog-cli-core/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"path/filepath"
	"testing"
)

const JfrogTestsHome = ".jfrogCliCoreTest"

// Prepare the .git environment for the test. Takes an existing folder and making it .git dir.
// sourceDirPath - Relative path to the source dir to change to .git
// targetDirPath - Relative path to the target created .git dir, usually 'testdata' under the parent dir.
func PrepareDotGitDir(t *testing.T, sourceDirPath, targetDirPath string) (string, string) {
	// Get path to create .git folder in
	baseDir, _ := os.Getwd()
	baseDir = filepath.Join(baseDir, targetDirPath)
	// Create .git path and make sure it is clean
	dotGitPath := filepath.Join(baseDir, ".git")
	RemovePath(dotGitPath, t)
	// Get the path of the .git candidate path
	dotGitPathTest := filepath.Join(baseDir, sourceDirPath)
	// Rename the .git candidate
	RenamePath(dotGitPathTest, dotGitPath, t)
	return baseDir, dotGitPath
}

// Removing the provided path from the filesystem
func RemovePath(testPath string, t *testing.T) {
	err := fileutils.RemovePath(testPath)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
}

// Renaming from old path to new path.
func RenamePath(oldPath, newPath string, t *testing.T) {
	err := fileutils.RenamePath(oldPath, newPath)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
}

// Set HomeDir to desired location.
// Caller is responsible to set the old home location back.
func SetJfrogHome() (oldHome string, err error) {
	homePath, err := filepath.Abs(JfrogTestsHome)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	homePath, err = filepath.Abs(homePath)
	if err != nil {
		return "", err
	}

	oldHome, err = coreutils.GetJfrogHomeDir()
	if err != nil {
		return "", err
	}

	err = os.Setenv(coreutils.HomeDir, homePath)
	if err != nil {
		return "", err
	}

	return oldHome, nil
}

func CleanUnitTestsJfrogHome() {
	homePath, err := filepath.Abs(JfrogTestsHome)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	errorOccurred := false
	if err := os.RemoveAll(homePath); err != nil {
		errorOccurred = true
		log.Error(err)
	}
	if err := os.Unsetenv(coreutils.HomeDir); err != nil {
		errorOccurred = true
		log.Error(err)
	}
	if errorOccurred {
		os.Exit(1)
	}
}
