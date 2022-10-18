package java

import (
	"fmt"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	gradleutils "github.com/jfrog/jfrog-cli-core/v2/utils/gradle"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

func BuildGradleDependencyTree(excludeTestDeps, useWrapper, ignoreConfigFile bool) (dependencyTree []*services.GraphNode, err error) {
	buildConfiguration, cleanBuild := createBuildConfiguration("audit-gradle")
	defer cleanBuild(err)

	err = runGradle(buildConfiguration, excludeTestDeps, useWrapper, ignoreConfigFile)
	if err != nil {
		return
	}

	dependencyTree, err = createGavDependencyTree(buildConfiguration)
	return
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
