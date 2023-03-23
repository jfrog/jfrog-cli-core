package java

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/build-info-go/build"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	remoteDepTreePath = "artifactory/oss-releases"
	gradlew           = "gradlew"
	depTreeInitFile   = "gradledeptree.init"
	depTreeOutputFile = "gradledeptree.out"
	depTreeInitScript = `initscript {
    repositories { %s
		mavenCentral()
    }
    dependencies {
        classpath 'com.jfrog:gradle-dep-tree:+'
    }
}

allprojects {
	repositories { %s
	}
    apply plugin: com.jfrog.GradleDepTree
}`
	artifactoryRepository = `
		maven {
			url "%s/%s"
			credentials {
				username = '%s'
				password = '%s'
			}
		}`
)

type gradleDepsMap map[string]any

func (gdg *gradleDepsMap) appendTree(jsonDepTree []byte) error {
	var rootNode map[string]any
	if err := json.Unmarshal(jsonDepTree, &rootNode); err != nil {
		return err
	}

	for gav, node := range rootNode["children"].(map[string]any) {
		if _, exists := (*gdg)[gav]; !exists {
			(*gdg)[gav] = node
		}
	}
	return nil
}

// The gradle-dep-tree generates a JSON representation for the dependencies for each gradle build file in the project.
// parseDepTreeFiles iterates over those JSONs, and append them to the map of dependencies in gradleDepsMap struct.
func (gdg *gradleDepsMap) parseDepTreeFiles(jsonFiles []byte) error {
	outputFiles := strings.Split(strings.TrimSpace(string(jsonFiles)), "\n")
	for _, path := range outputFiles {
		tree, err := os.ReadFile(strings.TrimSpace(path))
		if err != nil {
			return err
		}
		if err = gdg.appendTree(tree); err != nil {
			return err
		}

	}
	return nil
}

type depTreeManager struct {
	server       *config.ServerDetails
	releasesRepo string
	depsRepo     string
	useWrapper   bool
}

func buildGradleDependencyTree(useWrapper bool, server *config.ServerDetails, depsRepo, releasesRepo string) (dependencyTree []*services.GraphNode, err error) {
	if (server != nil && server.IsEmpty()) || depsRepo == "" {
		depsRepo, server, err = getGradleConfig()
		if err != nil {
			return
		}
	}

	manager := &depTreeManager{
		server:       server,
		releasesRepo: releasesRepo,
		depsRepo:     depsRepo,
		useWrapper:   useWrapper,
	}

	outputFileContent, err := manager.runGradleDepTree()
	if err != nil {
		return nil, err
	}
	return manager.getGraphFromDepTree(outputFileContent)
}

func (dtp *depTreeManager) runGradleDepTree() (outputFileContent []byte, err error) {
	// Create the script file in the repository
	if err = dtp.createDepTreeScript(); err != nil {
		return
	}
	defer func() {
		e := os.Remove(depTreeInitFile)
		if err == nil {
			err = e
		}
	}()

	if dtp.useWrapper {
		dtp.useWrapper, err = isGradleWrapperExist()
		if err != nil {
			return
		}
	}

	return dtp.execGradleDepTree()
}

func (dtp *depTreeManager) createDepTreeScript() (err error) {
	depsRepo := ""
	releasesRepo := ""
	if dtp.server != nil {
		releasesRepo, err = getDepTreeArtifactoryRepository(fmt.Sprintf("%s/%s", dtp.releasesRepo, remoteDepTreePath), dtp.server)
		if err != nil {
			return
		}
		depsRepo, err = getDepTreeArtifactoryRepository(dtp.depsRepo, dtp.server)
		if err != nil {
			return
		}
	}
	depTreeInitScript := fmt.Sprintf(depTreeInitScript, releasesRepo, depsRepo)
	return os.WriteFile(depTreeInitFile, []byte(depTreeInitScript), 0666)
}

func (dtp *depTreeManager) execGradleDepTree() (outputFileContent []byte, err error) {
	gradleExecPath, err := build.GetGradleExecPath(dtp.useWrapper)
	if err != nil {
		return
	}
	if dtp.useWrapper {
		if err = os.Chmod(gradleExecPath, 0777); err != nil {
			return
		}
	}

	outputFileAbsolutePath, err := filepath.Abs(depTreeOutputFile)
	if err != nil {
		return nil, err
	}
	tasks := []string{"clean", "generateDepTrees", "-I", depTreeInitFile, "-q", fmt.Sprintf("-Dcom.jfrog.depsTreeOutputFile=%s", outputFileAbsolutePath), "-Dcom.jfrog.includeAllBuildFiles=true"}
	log.Info("Running gradle dep tree command: ", gradleExecPath, tasks)
	if output, err := exec.Command(gradleExecPath, tasks...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("error running gradle-dep-tree: %s\n%s", err.Error(), string(output))
	}
	defer func() {
		e := os.Remove(outputFileAbsolutePath)
		if err == nil {
			err = e
		}
	}()

	return os.ReadFile(depTreeOutputFile)
}

// Assuming we ran gradle-dep-tree, getGraphFromDepTree receives the content of the depTreeOutputFile as input
func (dtp *depTreeManager) getGraphFromDepTree(outputFileContent []byte) ([]*services.GraphNode, error) {
	dependencyMap := gradleDepsMap{}
	if err := dependencyMap.parseDepTreeFiles(outputFileContent); err != nil {
		return nil, err
	}

	var depsGraph []*services.GraphNode
	for dependency, dependencyDetails := range dependencyMap {
		directDependency := &services.GraphNode{
			Id:    GavPackageTypeIdentifier + dependency,
			Nodes: []*services.GraphNode{},
		}
		populateGradleDependencyTree(directDependency, dependencyDetails.(map[string]any)["children"].(map[string]any))
		depsGraph = append(depsGraph, directDependency)
	}
	return depsGraph, nil
}

func populateGradleDependencyTree(currNode *services.GraphNode, currNodeChildren map[string]any) {
	if currNode.NodeHasLoop() {
		return
	}

	for gav, details := range currNodeChildren {
		childNode := &services.GraphNode{
			Id:     GavPackageTypeIdentifier + gav,
			Nodes:  []*services.GraphNode{},
			Parent: currNode,
		}
		childNodeChildren := details.(map[string]any)["children"].(map[string]any)
		populateGradleDependencyTree(childNode, childNodeChildren)
		currNode.Nodes = append(currNode.Nodes, childNode)
	}
}

func getDepTreeArtifactoryRepository(remoteRepo string, server *config.ServerDetails) (string, error) {
	pass := server.Password
	user := server.User
	if server.AccessToken != "" {
		pass = server.AccessToken
	}
	if pass == "" && user == "" {
		return "", fmt.Errorf("either username/password or access token must be set for %s", server.Url)
	}
	return fmt.Sprintf(artifactoryRepository,
		server.ArtifactoryUrl,
		remoteRepo,
		user,
		pass), nil
}

func getGradleConfig() (string, *config.ServerDetails, error) {
	var exists bool
	configFilePath, exists, err := utils.GetProjectConfFilePath(utils.Gradle)
	if err != nil {
		return "", nil, err
	}
	if !exists {
		return "", nil, nil
	}
	log.Debug("Using resolver config from", configFilePath)
	configContent, err := utils.ReadConfigFile(configFilePath, utils.YAML)
	if err != nil {
		return "", nil, err
	}
	var repository string
	if configContent.IsSet("resolver.repo") {
		repository = configContent.Get("resolver.repo").(string)
	}
	server, err := utils.GetServerDetails(configContent)
	return repository, server, err
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
