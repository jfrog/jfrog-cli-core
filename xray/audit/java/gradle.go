package java

import (
	"fmt"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	ioUtils "github.com/jfrog/jfrog-client-go/utils/io"

	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	gradleutils "github.com/jfrog/jfrog-cli-core/v2/utils/gradle"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

func AuditGradle(xrayGraphScanPrams services.XrayGraphScanParams, serverDetails *config.ServerDetails, excludeTestDeps, useWrapper, ignoreConfigFile bool, progress ioUtils.ProgressMgr) (results []services.ScanResponse, isMultipleRootProject bool, err error) {
	graph, err := BuildGradleDependencyTree(excludeTestDeps, useWrapper, ignoreConfigFile)
	if err != nil {
		return
	}
	isMultipleRootProject = len(graph) > 1
	results, err = audit.Scan(graph, xrayGraphScanPrams, serverDetails, progress, coreutils.Gradle)
	return
}

func BuildGradleDependencyTree(excludeTestDeps, useWrapper, ignoreConfigFile bool) (modules []*services.GraphNode, err error) {
	buildConfiguration, cleanBuild := createBuildConfiguration("audit-gradle")
	defer cleanBuild(err)

	err = runGradle(buildConfiguration, excludeTestDeps, useWrapper, ignoreConfigFile)
	if err != nil {
		return
	}

	return createGavDependencyTree(buildConfiguration)
}

func runGradle(buildConfiguration *utils.BuildConfiguration, excludeTestDeps, useWrapper, ignoreConfigFile bool) (err error) {
	tasks := "clean compileJava "
	if !excludeTestDeps {
		tasks += "compileTestJava "
	}
	tasks += "artifactoryPublish"
	log.Debug(fmt.Sprintf("gradle command tasks: %v", tasks))
	configFilePath := ""
	if !ignoreConfigFile {
		var exists bool
		configFilePath, exists, err = utils.GetProjectConfFilePath(utils.Gradle)
		if err != nil {
			return
		}
		if exists {
			log.Debug("Using resolver config from", configFilePath)
		}
	}
	// Read config
	vConfig, err := utils.ReadGradleConfig(configFilePath, useWrapper)
	if err != nil {
		return err
	}
	return gradleutils.RunGradle(vConfig, tasks, "", buildConfiguration, 0, useWrapper, true)
}
