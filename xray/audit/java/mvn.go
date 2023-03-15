package java

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	mvnutils "github.com/jfrog/jfrog-cli-core/v2/utils/mvn"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

func buildMvnDependencyTree(insecureTls, ignoreConfigFile, useWrapper bool, mvnProps map[string]any) (modules []*services.GraphNode, err error) {
	buildConfiguration, cleanBuild := createBuildConfiguration("audit-mvn")
	defer cleanBuild(err)

	err = runMvn(buildConfiguration, insecureTls, ignoreConfigFile, useWrapper, mvnProps)
	if err != nil {
		return
	}

	return createGavDependencyTree(buildConfiguration)
}

func runMvn(buildConfiguration *utils.BuildConfiguration, insecureTls, ignoreConfigFile, useWrapper bool, mvnProps map[string]any) (err error) {
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
		useWrapper, err = isMavenWrapperExist()
		if err != nil {
			return
		}
		if mvnProps == nil {
			mvnProps = make(map[string]any)
		}
		mvnProps["useWrapper"] = useWrapper
	}
	// Read config
	vConfig, err := utils.ReadMavenConfig(configFilePath, mvnProps)
	if err != nil {
		return err
	}
	return mvnutils.RunMvn(vConfig, "", buildConfiguration, goals, 0, insecureTls, true)
}

func isMavenWrapperExist() (bool, error) {
	wrapperName := "mvnw"
	if coreutils.IsWindows() {
		wrapperName += ".cmd"
	}
	return fileutils.IsFileExists(wrapperName, false)
}
