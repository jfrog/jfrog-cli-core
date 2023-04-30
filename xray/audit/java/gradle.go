package java

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jfrog/build-info-go/build"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
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
    }
    dependencies {
        classpath 'com.jfrog:gradle-dep-tree:2.2.0'
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

type depTreeManager struct {
	dependenciesTree
	server       *config.ServerDetails
	releasesRepo string
	depsRepo     string
	useWrapper   bool
}

// dependenciesTree represents a map between dependencies to their children dependencies in multiple projects.
type dependenciesTree struct {
	tree map[string][]dependenciesPaths
}

// dependenciesPaths represents a map between dependencies to their children dependencies in a single project.
type dependenciesPaths struct {
	Paths map[string]dependenciesPaths `json:"children"`
}

// The gradle-dep-tree generates a JSON representation for the dependencies for each gradle build file in the project.
// parseDepTreeFiles iterates over those JSONs, and append them to the map of dependencies in dependenciesTree struct.
func (dtp *depTreeManager) parseDepTreeFiles(jsonFiles []byte) error {
	outputFiles := strings.Split(strings.TrimSpace(string(jsonFiles)), "\n")
	for _, path := range outputFiles {
		tree, err := os.ReadFile(strings.TrimSpace(path))
		if err != nil {
			return errorutils.CheckError(err)
		}

		encodedFileName := path[strings.LastIndex(path, string(os.PathSeparator))+1:]
		decodedFileName, err := base64.StdEncoding.DecodeString(encodedFileName)
		if err != nil {
			return errorutils.CheckError(err)
		}

		if err = dtp.appendDependenciesPaths(tree, string(decodedFileName)); err != nil {
			return errorutils.CheckError(err)
		}
	}
	return nil
}

func (dtp *depTreeManager) appendDependenciesPaths(jsonDepTree []byte, fileName string) error {
	var deps dependenciesPaths
	if err := json.Unmarshal(jsonDepTree, &deps); err != nil {
		return errorutils.CheckError(err)
	}
	if dtp.tree == nil {
		dtp.tree = make(map[string][]dependenciesPaths)
	}
	dtp.tree[fileName] = append(dtp.tree[fileName], deps)
	return nil
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
	depTreeDir, err := dtp.createDepTreeScript()
	if err != nil {
		return
	}
	defer func() {
		e := fileutils.RemoveTempDir(depTreeDir)
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

	return dtp.execGradleDepTree(depTreeDir)
}

func (dtp *depTreeManager) createDepTreeScript() (tmpDir string, err error) {
	tmpDir, err = fileutils.CreateTempDir()
	if err != nil {
		return
	}
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
	return tmpDir, errorutils.CheckError(os.WriteFile(filepath.Join(tmpDir, depTreeInitFile), []byte(depTreeInitScript), 0666))
}

func (dtp *depTreeManager) execGradleDepTree(depTreeDir string) (outputFileContent []byte, err error) {
	gradleExecPath, err := build.GetGradleExecPath(dtp.useWrapper)
	if err != nil {
		err = errorutils.CheckError(err)
		return
	}

	outputFilePath := filepath.Join(depTreeDir, depTreeOutputFile)
	tasks := []string{
		"clean",
		"generateDepTrees", "-I", filepath.Join(depTreeDir, depTreeInitFile),
		"-q",
		fmt.Sprintf("-Dcom.jfrog.depsTreeOutputFile=%s", outputFilePath),
		"-Dcom.jfrog.includeAllBuildFiles=true"}
	log.Info("Running gradle deps tree command:", gradleExecPath, strings.Join(tasks, " "))
	if output, err := exec.Command(gradleExecPath, tasks...).CombinedOutput(); err != nil {
		return nil, errorutils.CheckErrorf("error running gradle-dep-tree: %s\n%s", err.Error(), string(output))
	}
	defer func() {
		e := errorutils.CheckError(os.Remove(outputFilePath))
		if err == nil {
			err = e
		}
	}()

	outputFileContent, err = os.ReadFile(outputFilePath)
	err = errorutils.CheckError(err)
	return
}

// Assuming we ran gradle-dep-tree, getGraphFromDepTree receives the content of the depTreeOutputFile as input
func (dtp *depTreeManager) getGraphFromDepTree(outputFileContent []byte) ([]*services.GraphNode, error) {
	if err := dtp.parseDepTreeFiles(outputFileContent); err != nil {
		return nil, err
	}
	var depsGraph []*services.GraphNode
	for dependency, children := range dtp.tree {
		directDependency := &services.GraphNode{
			Id:    GavPackageTypeIdentifier + dependency,
			Nodes: []*services.GraphNode{},
		}
		for _, childPath := range children {
			populateGradleDependencyTree(directDependency, childPath)
		}
		depsGraph = append(depsGraph, directDependency)
	}
	return depsGraph, nil
}

func populateGradleDependencyTree(currNode *services.GraphNode, currNodeChildren dependenciesPaths) {
	for gav, children := range currNodeChildren.Paths {
		childNode := &services.GraphNode{
			Id:     GavPackageTypeIdentifier + gav,
			Nodes:  []*services.GraphNode{},
			Parent: currNode,
		}
		if currNode.NodeHasLoop() {
			return
		}
		populateGradleDependencyTree(childNode, children)
		currNode.Nodes = append(currNode.Nodes, childNode)
	}
}

func getDepTreeArtifactoryRepository(remoteRepo string, server *config.ServerDetails) (string, error) {
	pass := server.Password
	user := server.User
	if server.AccessToken != "" {
		pass = server.AccessToken
		if user == "" {
			user = auth.ExtractUsernameFromAccessToken(pass)
		}
	}
	if pass == "" && user == "" {
		errString := "either username/password or access token must be set for "
		if server.Url != "" {
			errString += server.Url
		} else {
			errString += server.ArtifactoryUrl
		}
		return "", errors.New(errString)
	}
	return fmt.Sprintf(artifactoryRepository,
		strings.TrimSuffix(server.ArtifactoryUrl, "/"),
		remoteRepo,
		user,
		pass), nil
}

// getGradleConfig the remote repository and server details defined in the .jfrog/projects/gradle.yaml file, if configured.
func getGradleConfig() (string, *config.ServerDetails, error) {
	var exists bool
	configFilePath, exists, err := utils.GetProjectConfFilePath(utils.Gradle)
	if err != nil || !exists {
		return "", nil, err
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
