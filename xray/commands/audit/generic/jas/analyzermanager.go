package jas

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"os/exec"
)

type AnalyzerManager interface {
	DoesAnalyzerManagerExecutableExist() bool
	RunAnalyzerManager(string, string) error
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

func (am *analyzerManager) RunAnalyzerManager(configFile string, scanCommand string) error {
	analyzerManagerPath, err := getAnalyzerManagerAbsolutePath()
	if err != nil {
		return err
	}
	if coreutils.IsWindows() {
		err = exec.Command(analyzerManagerPath+".exe", scanCommand, configFile).Run()
	} else {
		err = exec.Command(analyzerManagerPath, scanCommand, configFile).Run()
	}
	return err
}
