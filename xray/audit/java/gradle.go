package java

import (
	"fmt"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"os"
	"path/filepath"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	gradleutils "github.com/jfrog/jfrog-cli-core/v2/utils/gradle"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

const (
	gradlew    = "gradlew"
	gradlewbat = "gradlew.bat"
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
	// check if gradle wrapper exist
	if useWrapper {
		var wrapperExist bool
		wrapperExist, err = verifyGradleWrapper()
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

func verifyGradleWrapper() (bool, error) {
	wd, err := os.Getwd()
	if err != nil {
		return false, err
	}
	filesInDir, err := fileutils.ListFiles(wd, false)
	fullPathGradlew, err := filepath.Abs(gradlew)
	fullPathGradlewbat, err := filepath.Abs(gradlewbat)
	for _, file := range filesInDir {
		if file == fullPathGradlew || file == fullPathGradlewbat {
			return true, nil
		}
	}

	return false, nil
}
