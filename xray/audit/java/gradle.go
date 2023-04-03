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

func buildGradleDependencyTree(excludeTestDeps, useWrapper, ignoreConfigFile bool, gradleConfigParams map[string]any) (dependencyTree []*services.GraphNode, err error) {
	buildConfiguration, cleanBuild := createBuildConfiguration("audit-gradle")
	defer func() {
		e := cleanBuild()
		if err == nil {
			err = e
		}
	}()

	err = runGradle(buildConfiguration, excludeTestDeps, useWrapper, ignoreConfigFile, gradleConfigParams)
	if err != nil {
		return
	}

	dependencyTree, err = createGavDependencyTree(buildConfiguration)
	return
}

func runGradle(buildConfiguration *utils.BuildConfiguration, excludeTestDeps, useWrapper, ignoreConfigFile bool, gradleConfigParams map[string]any) (err error) {
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
	// Check whether gradle wrapper exists
	if useWrapper {
		useWrapper, err = isGradleWrapperExist()
		if err != nil {
			return
		}
		if gradleConfigParams == nil {
			gradleConfigParams = make(map[string]any)
		}
		gradleConfigParams["usewrapper"] = useWrapper
	}
	// Read config
	vConfig, err := utils.ReadGradleConfig(configFilePath, gradleConfigParams)
	if err != nil {
		return err
	}
	return gradleutils.RunGradle(vConfig, tasks, "", buildConfiguration, 0, true)
}

// This function assumes that the Gradle wrapper is in the root directory.
// The --project-dir option of Gradle won't work in this case.
func isGradleWrapperExist() (bool, error) {
	wrapperName := gradlew
	if coreutils.IsWindows() {
		wrapperName += ".bat"
	}
	return fileutils.IsFileExists(wrapperName, false)
}
