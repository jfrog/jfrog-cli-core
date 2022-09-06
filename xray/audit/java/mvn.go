package java

import (
	"fmt"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	ioUtils "github.com/jfrog/jfrog-client-go/utils/io"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	mvnutils "github.com/jfrog/jfrog-cli-core/v2/utils/mvn"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

func AuditMvn(xrayGraphScanPrams services.XrayGraphScanParams, serverDetails *config.ServerDetails, insecureTls, ignoreConfigFile bool, progress ioUtils.ProgressMgr) (results []services.ScanResponse, isMultipleRootProject bool, err error) {
	graph, err := BuildMvnDependencyTree(insecureTls, ignoreConfigFile)
	if err != nil {
		return
	}
	isMultipleRootProject = len(graph) > 1
	results, err = audit.Scan(graph, xrayGraphScanPrams, serverDetails, progress, coreutils.Maven.ToString())
	return
}

func BuildMvnDependencyTree(insecureTls, ignoreConfigFile bool) (modules []*services.GraphNode, err error) {
	buildConfiguration, cleanBuild := createBuildConfiguration("audit-mvn")
	defer cleanBuild(err)

	err = runMvn(buildConfiguration, insecureTls, ignoreConfigFile)
	if err != nil {
		return
	}

	return createGavDependencyTree(buildConfiguration)
}

func runMvn(buildConfiguration *utils.BuildConfiguration, insecureTls, ignoreConfigFile bool) (err error) {
	goals := []string{"-B", "compile", "test-compile"}
	log.Debug(fmt.Sprintf("mvn command goals: %v", goals))
	configFilePath := ""
	if !ignoreConfigFile {
		var exists bool
		configFilePath, exists, err = utils.GetProjectConfFilePath(utils.Maven)
		if err != nil {
			return
		}
		if exists {
			log.Debug("Using resolver config from " + configFilePath)
		}
	}
	// Read config
	vConfig, err := utils.ReadMavenConfig(configFilePath)
	if err != nil {
		return err
	}
	return mvnutils.RunMvn(vConfig, "", buildConfiguration, goals, 0, insecureTls, true)
}
