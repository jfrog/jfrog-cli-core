package jas

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"os/exec"
)

type AnalyzerManager interface {
	DoesAnalyzerManagerExecutableExist() bool
	RunAnalyzerManager(string) error
}

type analyzerManager struct {
}

func (am *analyzerManager) DoesAnalyzerManagerExecutableExist() bool {
	analyzerManagerPath, err := getAnalyzerManagerAbsolutePath()
	if err != nil {
		return false
	}
	if exist, _ := fileutils.IsFileExists(analyzerManagerPath, false); exist {
		return false
	}
	return true
}

func (am *analyzerManager) RunAnalyzerManager(configFile string) error {
	analyzerManagerPath, err := getAnalyzerManagerAbsolutePath()
	if err != nil {
		return err
	}
	if coreutils.IsWindows() {
		err = exec.Command(analyzerManagerPath+".exe", applicabilityScanCommand, configFile).Run()
	} else {
		err = exec.Command(analyzerManagerPath, applicabilityScanCommand, configFile).Run()
	}
	return err
}
