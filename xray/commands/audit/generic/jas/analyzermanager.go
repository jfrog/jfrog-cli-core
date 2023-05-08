package jas

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"os/exec"
)

type AnalyzerManager interface {
	DoesAnalyzerManagerExecutableExist() (bool, error)
	RunAnalyzerManager(string) error
}

type analyzerManager struct {
}

func (am *analyzerManager) DoesAnalyzerManagerExecutableExist() (bool, error) {
	analyzerManagerPath, err := getAnalyzerManagerAbsolutePath()
	if err != nil {
		return false, err
	}
	exist, err := fileutils.IsFileExists(analyzerManagerPath, false)
	if err != nil {
		return false, err
	}
	if exist {
		return true, nil
	}
	return false, nil
}

func (am *analyzerManager) RunAnalyzerManager(configFile string) error {
	analyzerManagerPath, err := getAnalyzerManagerAbsolutePath()
	if err != nil {
		return err
	}
	if coreutils.IsWindows() {
		windowsExecutable := analyzerManagerPath + ".exe"
		err = exec.Command(windowsExecutable, applicabilityScanCommand, configFile).Run()
	} else {
		err = exec.Command(analyzerManagerPath, applicabilityScanCommand, configFile).Run()
	}
	return err
}
