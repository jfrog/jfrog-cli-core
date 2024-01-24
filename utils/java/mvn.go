package java

import (
	_ "embed"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
)

const (
	mavenDepTreeJarFile    = "maven-dep-tree.jar"
	mavenDepTreeOutputFile = "mavendeptree.out"
	// Changing this version also requires a change in MAVEN_DEP_TREE_VERSION within buildscripts/download_jars.sh
	mavenDepTreeVersion = "1.0.2"
	settingsXmlFile     = "settings.xml"
)

var mavenConfigPath = filepath.Join(".mvn", "maven.config")

type MavenDepTreeCmd string

const (
	Projects MavenDepTreeCmd = "projects"
	Tree     MavenDepTreeCmd = "tree"
)

//go:embed resources/settings.xml
var settingsXmlTemplate string

//go:embed resources/maven-dep-tree.jar
var mavenDepTreeJar []byte

type MavenDepTreeManager struct {
	DepTreeManager
	isInstalled     bool
	cmdName         MavenDepTreeCmd
	settingsXmlPath string
}

func NewMavenDepTreeManager(params *DepTreeParams, cmdName MavenDepTreeCmd, isDepTreeInstalled bool) *MavenDepTreeManager {
	depTreeManager := NewDepTreeManager(&DepTreeParams{
		Server:   params.Server,
		DepsRepo: params.DepsRepo,
	})
	return &MavenDepTreeManager{
		DepTreeManager: depTreeManager,
		isInstalled:    isDepTreeInstalled,
		cmdName:        cmdName,
	}
}

func buildMavenDependencyTree(params *DepTreeParams, isDepTreeInstalled bool) (dependencyTree []*xrayUtils.GraphNode, uniqueDeps []string, err error) {
	manager := NewMavenDepTreeManager(params, Tree, isDepTreeInstalled)
	outputFilePaths, clearMavenDepTreeRun, err := manager.RunMavenDepTree()
	if err != nil {
		if clearMavenDepTreeRun != nil {
			err = errors.Join(err, clearMavenDepTreeRun())
		}
		return
	}

	defer func() {
		err = errors.Join(err, clearMavenDepTreeRun())
	}()

	dependencyTree, uniqueDeps, err = getGraphFromDepTree(outputFilePaths)
	return
}

// Runs maven-dep-tree according to cmdName. Returns the plugin output along with a function pointer to revert the plugin side effects.
// If a non-nil clearMavenDepTreeRun pointer is returnes it means we had no error during the entire function execution
func (mdt *MavenDepTreeManager) RunMavenDepTree() (depTreeOutput string, clearMavenDepTreeRun func() error, err error) {
	// depTreeExecDir is a temp directory for all the files that are required for the maven-dep-tree run
	depTreeExecDir, clearMavenDepTreeRun, err := mdt.CreateTempDirWithSettingsXmlIfNeeded()
	if err != nil {
		if clearMavenDepTreeRun != nil {
			err = errors.Join(err, clearMavenDepTreeRun())
		}
		return
	}
	if err = mdt.installMavenDepTreePlugin(depTreeExecDir); err != nil {
		return
	}

	depTreeOutput, err = mdt.execMavenDepTree(depTreeExecDir)
	if err != nil {
		return
	}
	return
}

func (mdt *MavenDepTreeManager) installMavenDepTreePlugin(depTreeExecDir string) error {
	if mdt.isInstalled {
		return nil
	}
	mavenDepTreeJarPath := filepath.Join(depTreeExecDir, mavenDepTreeJarFile)
	if err := errorutils.CheckError(os.WriteFile(mavenDepTreeJarPath, mavenDepTreeJar, 0666)); err != nil {
		return err
	}
	goals := GetMavenPluginInstallationGoals(mavenDepTreeJarPath)
	_, err := mdt.RunMvnCmd(goals)
	return err
}

func GetMavenPluginInstallationGoals(pluginPath string) []string {
	return []string{"org.apache.maven.plugins:maven-install-plugin:3.1.1:install-file", "-Dfile=" + pluginPath, "-B"}
}

func (mdt *MavenDepTreeManager) execMavenDepTree(depTreeExecDir string) (string, error) {
	if mdt.cmdName == Tree {
		return mdt.runTreeCmd(depTreeExecDir)
	}
	return mdt.runProjectsCmd()
}

func (mdt *MavenDepTreeManager) runTreeCmd(depTreeExecDir string) (string, error) {
	mavenDepTreePath := filepath.Join(depTreeExecDir, mavenDepTreeOutputFile)
	goals := []string{"com.jfrog:maven-dep-tree:" + mavenDepTreeVersion + ":" + string(Tree), "-DdepsTreeOutputFile=" + mavenDepTreePath, "-B"}
	if _, err := mdt.RunMvnCmd(goals); err != nil {
		return "", err
	}

	mavenDepTreeOutput, err := os.ReadFile(mavenDepTreePath)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return string(mavenDepTreeOutput), nil
}

func (mdt *MavenDepTreeManager) runProjectsCmd() (string, error) {
	goals := []string{"com.jfrog:maven-dep-tree:" + mavenDepTreeVersion + ":" + string(Projects), "-q"}
	output, err := mdt.RunMvnCmd(goals)
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func (mdt *MavenDepTreeManager) RunMvnCmd(goals []string) (cmdOutput []byte, err error) {
	restoreMavenConfig, err := removeMavenConfig()
	if err != nil {
		return
	}

	defer func() {
		if restoreMavenConfig != nil {
			err = errors.Join(err, restoreMavenConfig())
		}
	}()

	if mdt.settingsXmlPath != "" {
		goals = append(goals, "-s", mdt.settingsXmlPath)
	}

	//#nosec G204
	cmdOutput, err = exec.Command("mvn", goals...).CombinedOutput()
	if err != nil {
		if len(cmdOutput) > 0 {
			log.Info(string(cmdOutput))
		}
		err = fmt.Errorf("failed running command 'mvn %s': %s", strings.Join(goals, " "), err.Error())
	}
	return
}

func (mdt *MavenDepTreeManager) GetSettingsXmlPath() string {
	return mdt.settingsXmlPath
}

func (mdt *MavenDepTreeManager) SetSettingsXmlPath(settingsXmlPath string) {
	mdt.settingsXmlPath = settingsXmlPath
}

func removeMavenConfig() (func() error, error) {
	mavenConfigExists, err := fileutils.IsFileExists(mavenConfigPath, false)
	if err != nil {
		return nil, err
	}
	if !mavenConfigExists {
		return nil, nil
	}
	restoreMavenConfig, err := ioutils.BackupFile(mavenConfigPath, "maven.config.bkp")
	if err != nil {
		return nil, err
	}
	err = os.Remove(mavenConfigPath)
	if err != nil {
		err = errorutils.CheckErrorf("failed to remove %s while building the maven dependencies tree. Error received:\n%s", mavenConfigPath, err.Error())
	}
	return restoreMavenConfig, err
}

// Creates a new settings.xml file configured with the provided server and repository from the current MavenDepTreeManager instance.
// The settings.xml will be written to the given path.
func (mdt *MavenDepTreeManager) createSettingsXmlWithConfiguredArtifactory(path string) error {
	username, password, err := getArtifactoryAuthFromServer(mdt.server)
	if err != nil {
		return err
	}
	remoteRepositoryFullPath, err := url.JoinPath(mdt.server.ArtifactoryUrl, mdt.depsRepo)
	if err != nil {
		return err
	}
	mdt.settingsXmlPath = filepath.Join(path, settingsXmlFile)
	settingsXmlContent := fmt.Sprintf(settingsXmlTemplate, username, password, remoteRepositoryFullPath)

	return errorutils.CheckError(os.WriteFile(mdt.settingsXmlPath, []byte(settingsXmlContent), 0600))
}

// Creates a temporary directory.
// If Artifactory resolution repo is provided, a settings.xml file with the provided server and repository will be created inside the temprary directory.
func (mdt *MavenDepTreeManager) CreateTempDirWithSettingsXmlIfNeeded() (tempDirPath string, clearMavenDepTreeRun func() error, err error) {
	tempDirPath, err = fileutils.CreateTempDir()
	if err != nil {
		return
	}

	clearMavenDepTreeRun = func() error { return fileutils.RemoveTempDir(tempDirPath) }

	// Create a settings.xml file that sets the dependency resolution from the given server and repository
	if mdt.depsRepo != "" {
		err = mdt.createSettingsXmlWithConfiguredArtifactory(tempDirPath)
	}
	return
}
