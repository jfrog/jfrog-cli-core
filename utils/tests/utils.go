package tests

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	corelog "github.com/jfrog/jfrog-cli-core/v2/utils/log"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

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
