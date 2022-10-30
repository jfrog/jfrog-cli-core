package java

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	gradleutils "github.com/jfrog/jfrog-cli-core/v2/utils/gradle"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

const gradlew = "gradlew"

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
	// check if gradle wrapper exist
	if useWrapper {
		var wrapperExist bool
		wrapperExist, err = isGradleWrapperExist()
		if err != nil {
			return
		}
		useWrapper = wrapperExist
	}
	// Read config
	vConfig, err := utils.ReadGradleConfig(configFilePath, useWrapper)
	if err != nil {
		return err
	}
	return gradleutils.RunGradle(vConfig, tasks, "", buildConfiguration, 0, useWrapper, true)
}

// This function assumes that the Gradle wrapper is in the root directory.
// Adapting this function is needed if the audit command supports Gradle's --project-dir option.
func isGradleWrapperExist() (bool, error) {
	wrapperName := gradlew
	if coreutils.IsWindows() {
		wrapperName += ".bat"
	}
	return fileutils.IsFileExists(wrapperName, false)
}
