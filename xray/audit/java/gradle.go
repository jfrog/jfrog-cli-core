package java

import (
	"fmt"
	ioUtils "github.com/jfrog/jfrog-client-go/utils/io"

	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	gradleutils "github.com/jfrog/jfrog-cli-core/v2/utils/gradle"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

func AuditGradle(xrayGraphScanPrams services.XrayGraphScanParams, serverDetails *config.ServerDetails, excludeTestDeps, useWrapper bool, progress ioUtils.ProgressMgr) (results []services.ScanResponse, isMultipleRootProject bool, err error) {
	graph, err := BuildGradleDependencyTree(excludeTestDeps, useWrapper)
	if err != nil {
		return
	}
	isMultipleRootProject = len(graph) > 1
	results, err = audit.Scan(graph, xrayGraphScanPrams, serverDetails, progress)
	return
}

func BuildGradleDependencyTree(excludeTestDeps, useWrapper bool) (modules []*services.GraphNode, err error) {
	buildConfiguration, cleanBuild := createBuildConfiguration("audit-gradle")
	defer cleanBuild(err)

	err = runGradle(buildConfiguration, excludeTestDeps, useWrapper)
	if err != nil {
		return
	}

	return createGavDependencyTree(buildConfiguration)
}

func runGradle(buildConfiguration *utils.BuildConfiguration, excludeTestDeps, useWrapper bool) error {
	tasks := "clean compileJava "
	if !excludeTestDeps {
		tasks += "compileTestJava "
	}
	tasks += "artifactoryPublish"
	log.Debug(fmt.Sprintf("gradle command tasks: %v", tasks))
	configFilePath, exists, err := utils.GetProjectConfFilePath(utils.Gradle)
	if err != nil {
		return err
	}
	if exists {
		log.Debug("Using resolver config from " + configFilePath)
	} else {
		configFilePath = ""
	}
	return gradleutils.RunGradle(tasks, configFilePath, "", buildConfiguration, 0, useWrapper, true)
}
