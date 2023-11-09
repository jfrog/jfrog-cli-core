package java

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	mvnutils "github.com/jfrog/jfrog-cli-core/v2/utils/mvn"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	mavenDepTreeJarFile    = "maven-dep-tree.jar"
	mavenDepTreeOutputFile = "mavendeptree.out"
	mavenDepTreeVersion    = "1.0.0"
	TreeCmd                = "tree"
	ProjectsCmd            = "projects"
)

//go:embed maven-dep-tree.jar
var mavenDepTreeJar []byte

type MavenDepTreeManager struct {
	*DepTreeManager
	cmdName     string
	isInstalled bool
}

func buildMavenDependencyTree(params *DepTreeParams) (dependencyTree []*xrayUtils.GraphNode, uniqueDeps []string, err error) {
	manager := &MavenDepTreeManager{DepTreeManager: NewDepTreeManager(params), cmdName: TreeCmd, isInstalled: params.IsMvnDepTreeInstalled}
	outputFileContent, err := manager.runMavenDepTree()
	if err != nil {
		return
	}
	dependencyTree, uniqueDeps, err = getGraphFromDepTree(outputFileContent)
	return
}

func (mdt *MavenDepTreeManager) runMavenDepTree() ([]byte, error) {
	depTreeExecDir, err := fileutils.CreateTempDir()
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, fileutils.RemoveTempDir(depTreeExecDir))
	}()
	if err = mdt.installMavenDepTreePlugin(depTreeExecDir); err != nil {
		return nil, err
	}
	return mdt.execMavenDepTree(depTreeExecDir)
}

func (mdt *MavenDepTreeManager) installMavenDepTreePlugin(depTreeExecDir string) error {
	if mdt.isInstalled {
		return nil
	}
	mavenDepTreeJarPath := filepath.Join(depTreeExecDir, mavenDepTreeJarFile)
	if err := errorutils.CheckError(os.WriteFile(mavenDepTreeJarPath, mavenDepTreeJar, 0666)); err != nil {
		return err
	}
	installArgs := GetMavenPluginInstallationArgs(mavenDepTreeJarPath)
	_, err := RunMvnCmd(mdt.depsRepo, mdt.server, installArgs)
	return err
}

func GetMavenPluginInstallationArgs(pluginPath string) []string {
	return []string{"org.apache.maven.plugins:maven-install-plugin:2.5.2:install-file", "-Dfile=" + pluginPath}
}

func (mdt *MavenDepTreeManager) execMavenDepTree(depTreeExecDir string) ([]byte, error) {
	mavenDepTreePath := filepath.Join(depTreeExecDir, mavenDepTreeOutputFile)
	goals := []string{"com.jfrog:maven-dep-tree:" + mavenDepTreeVersion + ":" + mdt.cmdName, "-DdepsTreeOutputFile=" + mavenDepTreePath}
	_, err := RunMvnCmd(mdt.depsRepo, mdt.server, goals)
	if err != nil {
		return nil, err
	}
	mavenDepTreeOutput, err := os.ReadFile(mavenDepTreePath)
	err = errorutils.CheckError(err)
	return mavenDepTreeOutput, err
}

func RunMvnCmd(depsRepo string, serverDetails *config.ServerDetails, goals []string) (cmdOutput []byte, err error) {
	if depsRepo != "" {
		// Run the mvn command with the Maven Build-Info Extractor to download dependencies from Artifactory.
		cmdOutput, err = runMvnCmdWithBuildInfoExtractor(depsRepo, serverDetails, goals)
		return
	}

	//#nosec G204
	if cmdOutput, err = exec.Command("mvn", goals...).CombinedOutput(); err != nil {
		if len(cmdOutput) > 0 {
			log.Info(string(cmdOutput))
		}
		err = fmt.Errorf("failed running command 'mvn %s': %s", strings.Join(goals, " "), err.Error())
	}
	return
}

// Run a Maven command with the specified goals
// utilizing the Maven Build-Info Extractor to fetch and download dependencies from the provided server and repository.
func runMvnCmdWithBuildInfoExtractor(depsRepo string, serverDetails *config.ServerDetails, goals []string) (cmdOutput []byte, err error) {
	mvnProps := createMvnProps(depsRepo, serverDetails)
	vConfig, err := utils.ReadMavenConfig("", mvnProps)
	if err != nil {
		return
	}
	var buf bytes.Buffer
	mvnParams := mvnutils.NewMvnUtils().
		SetConfig(vConfig).
		SetGoals(goals).
		SetDisableDeploy(true).
		SetOutputWriter(&buf)
	cmdOutput = make([]byte, 0)
	err = mvnutils.RunMvn(mvnParams)
	// cmdOutput should return from this function
	_, _ = io.ReadFull(&buf, cmdOutput)
	if err != nil {
		if len(cmdOutput) > 0 {
			// Log output if exists
			log.Info(string(cmdOutput))
		}
		err = fmt.Errorf("failed running command 'mvn %s': %s", strings.Join(goals, " "), err.Error())
	}
	return
}

func createMvnProps(resolverRepo string, serverDetails *config.ServerDetails) map[string]any {
	if serverDetails == nil || serverDetails.IsEmpty() {
		return nil
	}
	authPass := serverDetails.Password
	if serverDetails.AccessToken != "" {
		authPass = serverDetails.AccessToken
	}
	authUser := serverDetails.User
	if authUser == "" {
		authUser = auth.ExtractUsernameFromAccessToken(serverDetails.AccessToken)
	}
	return map[string]any{
		"resolver.username":                            authUser,
		"resolver.password":                            authPass,
		"resolver.url":                                 serverDetails.ArtifactoryUrl,
		"resolver.releaseRepo":                         resolverRepo,
		"resolver.snapshotRepo":                        resolverRepo,
		"buildInfoConfig.artifactoryResolutionEnabled": true,
	}
}
