package java

import (
	"fmt"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	mvnutils "github.com/jfrog/jfrog-cli-core/v2/utils/mvn"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

const mvnw = "mvnw"

func BuildMvnDependencyTree(insecureTls, ignoreConfigFile, useWrapper bool) (modules []*services.GraphNode, err error) {
	buildConfiguration, cleanBuild := createBuildConfiguration("audit-mvn")
	defer cleanBuild(err)

	err = runMvn(buildConfiguration, insecureTls, ignoreConfigFile, useWrapper)
	if err != nil {
		return
	}

	return createGavDependencyTree(buildConfiguration)
}

func runMvn(buildConfiguration *utils.BuildConfiguration, insecureTls, ignoreConfigFile, useWrapper bool) (err error) {
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
	if useWrapper {
		useWrapper, err = isMvnWrapperExist()
		if err != nil {
			return
		}
	}
	// Read config
	vConfig, err := utils.ReadMavenConfig(configFilePath, useWrapper)
	if err != nil {
		return err
	}
	return mvnutils.RunMvn(vConfig, "", buildConfiguration, goals, 0, insecureTls, useWrapper, true)
}

// This function assumes that the Maven wrapper is in the root directory.
// The --project-dir option of Maven won't work in this case.
func isMvnWrapperExist() (bool, error) {
	wrapperName := mvnw
	return fileutils.IsFileExists(wrapperName, false)
}
