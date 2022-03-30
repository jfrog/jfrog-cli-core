package python

import (
	"os"
	"path/filepath"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	piputils "github.com/jfrog/jfrog-cli-core/v2/utils/python"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

func AuditPip(xrayGraphScanPrams services.XrayGraphScanParams, serverDetails *config.ServerDetails) (results []services.ScanResponse, isMultipleRootProject bool, err error) {
	graph, err := BuildPipDependencyTree()
	if err != nil {
		return
	}
	isMultipleRootProject = len(graph) > 1
	results, err = audit.Scan(graph, xrayGraphScanPrams, serverDetails)
	return
}

func BuildPipDependencyTree() ([]*services.GraphNode, error) {
	dependenciesGraph, rootDependenciesList, err := getDependencies()
	if err != nil {
		return nil, err
	}
	var dependencyTree []*services.GraphNode
	for _, rootDep := range rootDependenciesList {
		parentNode := &services.GraphNode{
			Id:    pythonPackageTypeIdentifier + rootDep,
			Nodes: []*services.GraphNode{},
		}
		populatePythonDependencyTree(parentNode, dependenciesGraph)
		dependencyTree = append(dependencyTree, parentNode)
	}
	return dependencyTree, nil
}

func getDependencies() (dependenciesGraph map[string][]string, rootDependencies []string, err error) {
	wd, err := os.Getwd()
	if err != nil {
		return
	}

	// Create temp dir to run all work outside users working directory
	tempDirPath, err := fileutils.CreateTempDir()
	if err != nil {
		return
	}

	err = os.Chdir(tempDirPath)
	if err != nil {
		return
	}

	defer func() {
		e := os.Chdir(wd)
		if err == nil {
			err = e
		}

		e = fileutils.RemoveTempDir(tempDirPath)
		if err == nil {
			err = e
		}
	}()

	err = fileutils.CopyDir(wd, tempDirPath, true, nil)
	if err != nil {
		return
	}

	// 'virtualenv venv'
	err = piputils.RunVirtualEnv()
	if err != nil {
		return
	}

	// 'pip install .'
	err = piputils.RunPipInstall()
	if err != nil {
		exist, requirementsErr := fileutils.IsFileExists(filepath.Join(tempDirPath, "requirements.txt"), false)
		if requirementsErr != nil || !exist {
			return
		}

		log.Debug("Failed running 'pip install .' , trying 'pip install -r requirements.txt' ")
		requirementsErr = piputils.RunPipInstallRequirements()
		if requirementsErr != nil {
			log.Error(requirementsErr)
			return
		}
	}

	// Run pipdeptree.py to get dependencies tree
	dependenciesGraph, rootDependencies, err = piputils.RunPipDepTree(piputils.GetVenvPythonExecPath())
	return
}
