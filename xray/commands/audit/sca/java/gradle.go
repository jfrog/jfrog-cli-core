package java

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jfrog/build-info-go/build"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	remoteDepTreePath       = "artifactory/oss-release-local"
	gradlew                 = "gradlew"
	gradleDepTreeJarFile    = "gradle-dep-tree.jar"
	gradleDepTreeInitFile   = "gradledeptree.init"
	gradleDepTreeOutputFile = "gradledeptree.out"
	gradleDepTreeInitScript = `initscript {
	repositories { %s
		mavenCentral()
	}
	dependencies {
		classpath files('%s')
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

//go:embed resources/gradle-dep-tree.jar
var gradleDepTreeJar []byte

type gradleDepTreeManager struct {
	DepTreeManager
}

func buildFlatGradleDependencyTree(params *DepTreeParams) (uniqueDeps []string, err error) {
	manager := &gradleDepTreeManager{DepTreeManager: NewDepTreeManager(params)}
	outputFileContent, err := manager.runGradleDepTree()
	if err != nil {
		return
	}
	// TODO NOTICE change in functionality: create ONLY flat tree here
	uniqueDeps, err = getFlatGraphFromDepTree(outputFileContent)
	return
}

func (gdt *gradleDepTreeManager) runGradleDepTree() (string, error) {
	// Create the script file in the repository
	depTreeDir, err := gdt.createDepTreeScriptAndGetDir()
	if err != nil {
		return "", err
	}
	defer func() {
		err = errors.Join(err, fileutils.RemoveTempDir(depTreeDir))
	}()

	if gdt.useWrapper {
		gdt.useWrapper, err = isGradleWrapperExist()
		if err != nil {
			return "", err
		}
	}

	output, err := gdt.execGradleDepTree(depTreeDir)
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func (gdt *gradleDepTreeManager) createDepTreeScriptAndGetDir() (tmpDir string, err error) {
	tmpDir, err = fileutils.CreateTempDir()
	if err != nil {
		return
	}
	var releasesRepo string
	releasesRepo, gdt.depsRepo, err = getRemoteRepos(gdt.depsRepo, gdt.server)
	if err != nil {
		return
	}
	gradleDepTreeJarPath := filepath.Join(tmpDir, gradleDepTreeJarFile)
	if err = errorutils.CheckError(os.WriteFile(gradleDepTreeJarPath, gradleDepTreeJar, 0600)); err != nil {
		return
	}
	gradleDepTreeJarPath = ioutils.DoubleWinPathSeparator(gradleDepTreeJarPath)

	depTreeInitScript := fmt.Sprintf(gradleDepTreeInitScript, releasesRepo, gradleDepTreeJarPath, gdt.depsRepo)
	return tmpDir, errorutils.CheckError(os.WriteFile(filepath.Join(tmpDir, gradleDepTreeInitFile), []byte(depTreeInitScript), 0666))
}

// getRemoteRepos constructs the sections of Artifactory's remote repositories in the gradle-dep-tree init script.
// depsRemoteRepo - name of the remote repository that proxies the relevant registry, e.g. maven central.
// server - the Artifactory server details on which the repositories reside in.
// Returns the constructed sections.
func getRemoteRepos(depsRepo string, server *config.ServerDetails) (string, string, error) {
	constructedReleasesRepo, err := constructReleasesRemoteRepo()
	if err != nil {
		return "", "", err
	}

	constructedDepsRepo, err := getDepTreeArtifactoryRepository(depsRepo, server)
	if err != nil {
		return "", "", err
	}
	return constructedReleasesRepo, constructedDepsRepo, nil
}

func constructReleasesRemoteRepo() (string, error) {
	// Try to retrieve the serverID and remote repository that proxies https://releases.jfrog.io, from the environment variable
	serverId, repoName, err := coreutils.GetServerIdAndRepo(coreutils.ReleasesRemoteEnv)
	if err != nil || serverId == "" || repoName == "" {
		return "", err
	}

	releasesServer, err := config.GetSpecificConfig(serverId, false, true)
	if err != nil {
		return "", err
	}

	releasesPath := fmt.Sprintf("%s/%s", repoName, remoteDepTreePath)
	log.Debug("The `"+gradleDepTreeJarFile+"` will be resolved from", repoName)
	return getDepTreeArtifactoryRepository(releasesPath, releasesServer)
}

func (gdt *gradleDepTreeManager) execGradleDepTree(depTreeDir string) (outputFileContent []byte, err error) {
	gradleExecPath, err := build.GetGradleExecPath(gdt.useWrapper)
	if err != nil {
		err = errorutils.CheckError(err)
		return
	}

	outputFilePath := filepath.Join(depTreeDir, gradleDepTreeOutputFile)
	tasks := []string{
		"clean",
		"generateDepTrees", "-I", filepath.Join(depTreeDir, gradleDepTreeInitFile),
		"-q",
		fmt.Sprintf("-Dcom.jfrog.depsTreeOutputFile=%s", outputFilePath),
		"-Dcom.jfrog.includeAllBuildFiles=true"}
	log.Info("Running gradle deps tree command:", gradleExecPath, strings.Join(tasks, " "))
	if output, err := exec.Command(gradleExecPath, tasks...).CombinedOutput(); err != nil {
		return nil, errorutils.CheckErrorf("error running gradle-dep-tree: %s\n%s", err.Error(), string(output))
	}
	defer func() {
		err = errors.Join(err, errorutils.CheckError(os.Remove(outputFilePath)))
	}()

	outputFileContent, err = os.ReadFile(outputFilePath)
	err = errorutils.CheckError(err)
	return
}

func getDepTreeArtifactoryRepository(remoteRepo string, server *config.ServerDetails) (string, error) {
	if remoteRepo == "" || server.IsEmpty() {
		return "", nil
	}
	username, password, err := getArtifactoryAuthFromServer(server)
	if err != nil {
		return "", err
	}

	log.Debug("The project dependencies will be resolved from", server.ArtifactoryUrl, "from the", remoteRepo, "repository")
	return fmt.Sprintf(artifactoryRepository,
		strings.TrimSuffix(server.ArtifactoryUrl, "/"),
		remoteRepo,
		username,
		password), nil
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
