package java

import (
	"fmt"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	mvnutils "github.com/jfrog/jfrog-cli-core/v2/utils/mvn"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

func AuditMvn(xrayGraphScanPrams services.XrayGraphScanParams, serverDetails *config.ServerDetails, insecureTls bool) (results []services.ScanResponse, isMultipleRootProject bool, err error) {
	graph, err := BuildMvnDependencyTree(insecureTls)
	if err != nil {
		return
	}
	isMultipleRootProject = len(graph) > 1
	results, err = audit.Scan(graph, xrayGraphScanPrams, serverDetails)
	return
}

func BuildMvnDependencyTree(insecureTls bool) (modules []*services.GraphNode, err error) {
	buildConfiguration, cleanBuild := createBuildConfiguration("audit-mvn")
	defer cleanBuild(err)

	err = runMvn(buildConfiguration, insecureTls)
	if err != nil {
		return
	}

	return createGavDependencyTree(buildConfiguration)
}

func runMvn(buildConfiguration *utils.BuildConfiguration, insecureTls bool) error {
	goals := []string{"-B", "compile", "test-compile"}
	log.Debug(fmt.Sprintf("mvn command goals: %v", goals))
	configFilePath, exists, err := utils.GetProjectConfFilePath(utils.Maven)
	if err != nil {
		return err
	}
	if exists {
		log.Debug("Using resolver config from " + configFilePath)
	} else {
		configFilePath = ""
	}
	return mvnutils.RunMvn(configFilePath, "", buildConfiguration, goals, 0, insecureTls, true)
}
