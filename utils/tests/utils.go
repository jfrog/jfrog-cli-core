package tests

import (
	"bytes"
	"errors"
	"fmt"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	corelog "github.com/jfrog/jfrog-cli-core/v2/utils/log"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/stretchr/testify/assert"
)

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
func SetJfrogHome() (cleanUp func(), err error) {
	homePath, err := fileutils.CreateTempDir()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	homePath, err = filepath.Abs(homePath)
	if err != nil {
		return func() {}, err
	}

	err = os.Setenv(coreutils.HomeDir, homePath)
	if err != nil {
		return func() {}, err
	}

	return func() { cleanUpUnitTestsJfrogHome(homePath) }, nil
}

func cleanUpUnitTestsJfrogHome(homeDir string) {
	homePath, err := filepath.Abs(homeDir)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	errorOccurred := false
	if err := fileutils.RemoveTempDir(homePath); err != nil {
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

func CreateTempDirWithCallbackAndAssert(t *testing.T) (string, func()) {
	tempDirPath, err := fileutils.CreateTempDir()
	assert.NoError(t, err, "Couldn't create temp dir")
	return tempDirPath, func() {
		assert.NoError(t, fileutils.RemoveTempDir(tempDirPath), "Couldn't remove temp dir")
	}
}

func ValidateListsIdentical(expected, actual []string) error {
	if len(actual) != len(expected) {
		return fmt.Errorf("unexpected behavior, \nexpected: [%s], \nfound:    [%s]", strings.Join(expected, ", "), strings.Join(actual, ", "))
	}
	err := compare(expected, actual)
	return err
}

func compare(expected, actual []string) error {
	for _, v := range expected {
		for i, r := range actual {
			if v == r {
				break
			}
			if i == len(actual)-1 {
				return errors.New("Missing file : " + v)
			}
		}
	}
	return nil
}

// CompareTree returns true iff the two trees contain the same nodes (regardless of their order)
func CompareTree(expected, actual *xrayUtils.GraphNode) bool {
	if expected.Id != actual.Id {
		return false
	}
	// Make sure all children are equal, when order doesn't matter
	for _, expectedNode := range expected.Nodes {
		found := false
		for _, actualNode := range actual.Nodes {
			if CompareTree(expectedNode, actualNode) {
				found = true
				break
			}
		}
		// After iterating over all B's nodes, non match nodeA so the tree aren't equals.
		if !found {
			return false
		}
	}
	return true
}

// Set new logger with output redirection to a buffer.
// Caller is responsible to set the old log back.
func RedirectLogOutputToBuffer() (outputBuffer, stderrBuffer *bytes.Buffer, previousLog log.Log) {
	stderrBuffer, outputBuffer = &bytes.Buffer{}, &bytes.Buffer{}
	previousLog = log.Logger
	newLog := log.NewLogger(corelog.GetCliLogLevel(), nil)
	newLog.SetOutputWriter(outputBuffer)
	newLog.SetLogsWriter(stderrBuffer, 0)
	log.SetLogger(newLog)
	return outputBuffer, stderrBuffer, previousLog
}
