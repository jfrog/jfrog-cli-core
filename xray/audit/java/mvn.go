package java

import (
	"fmt"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	mvnutils "github.com/jfrog/jfrog-cli-core/v2/utils/mvn"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

func BuildMvnDependencyTree(insecureTls, ignoreConfigFile bool, mvnProps map[string]any) (modules []*services.GraphNode, err error) {
	buildConfiguration, cleanBuild := createBuildConfiguration("audit-mvn")
	defer cleanBuild(err)

	err = runMvn(buildConfiguration, insecureTls, ignoreConfigFile, mvnProps)
	if err != nil {
		return
	}

	return createGavDependencyTree(buildConfiguration)
}

func runMvn(buildConfiguration *utils.BuildConfiguration, insecureTls, ignoreConfigFile bool, mvnProps map[string]any) (err error) {
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
	vConfig, err := utils.ReadMavenConfig(configFilePath, mvnProps)
	if err != nil {
		return err
	}
	return mvnutils.RunMvn(vConfig, "", buildConfiguration, goals, 0, insecureTls, true)
}
