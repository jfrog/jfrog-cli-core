package java

import (
	_ "embed"
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	mavenDepTreeJarFile    = "maven-dep-tree.jar"
	mavenDepTreeOutputFile = "mavendeptree.out"
	mavenDepTreeVersion    = "1.0.2"
	TreeCmd                = "tree"
	ProjectsCmd            = "projects"
	settingsXmlFile        = "settings.xml"
)

var settingsXmlTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<settings xsi:schemaLocation="http://maven.apache.org/SETTINGS/1.2.0 http://maven.apache.org/xsd/settings-1.2.0.xsd" xmlns="http://maven.apache.org/SETTINGS/1.2.0"
    xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <servers>
    <server>
      <username>%s</username>
      <password>%s</password>
      <id>artifactory</id>
    </server>
  </servers>
  <mirrors>
    <mirror>
          <id>artifactory</id>
          <url>%s</url>
          <mirrorOf>*</mirrorOf>
    </mirror>
  </mirrors>
</settings>`

//go:embed maven-dep-tree.jar
var mavenDepTreeJar []byte

type MavenDepTreeManager struct {
	*DepTreeManager
	isInstalled     bool
	cmdName         string
	settingsXmlPath string
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
	// Create a temp directory for all the files that are required for the maven-dep-tree run
	depTreeExecDir, err := fileutils.CreateTempDir()
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, fileutils.RemoveTempDir(depTreeExecDir))
	}()

	// Create a settings.xml file that sets the dependency resolution from the given server and repository
	if mdt.depsRepo != "" {
		if err = mdt.createSettingsXmlWithConfiguredArtifactory(depTreeExecDir); err != nil {
			return nil, err
		}
	}
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
	goals := GetMavenPluginInstallationGoals(mavenDepTreeJarPath)
	return mdt.runMvnCmd(goals)
}

func GetMavenPluginInstallationGoals(pluginPath string) []string {
	return []string{"org.apache.maven.plugins:maven-install-plugin:2.5.2:install-file", "-Dfile=" + pluginPath}
}

func (mdt *MavenDepTreeManager) execMavenDepTree(depTreeExecDir string) ([]byte, error) {
	mavenDepTreePath := filepath.Join(depTreeExecDir, mavenDepTreeOutputFile)
	goals := []string{"com.jfrog:maven-dep-tree:" + mavenDepTreeVersion + ":" + mdt.cmdName, "-DdepsTreeOutputFile=" + mavenDepTreePath}
	if err := mdt.runMvnCmd(goals); err != nil {
		return nil, err
	}

	mavenDepTreeOutput, err := os.ReadFile(mavenDepTreePath)
	err = errorutils.CheckError(err)
	return mavenDepTreeOutput, err
}

func (mdt *MavenDepTreeManager) runMvnCmd(goals []string) error {
	if mdt.settingsXmlPath != "" {
		goals = append(goals, "-s", mdt.settingsXmlPath)
	}

	//#nosec G204
	cmdOutput, err := exec.Command("mvn", goals...).CombinedOutput()
	if err != nil {
		if len(cmdOutput) > 0 {
			log.Info(string(cmdOutput))
		}
		err = fmt.Errorf("failed running command 'mvn %s': %s", strings.Join(goals, " "), err.Error())
	}
	return err
}

// Creates a new settings.xml file configured with the provided server and repository from the current MavenDepTreeManager instance.
// The settings.xml will be written to the given path.
func (mdt *MavenDepTreeManager) createSettingsXmlWithConfiguredArtifactory(path string) error {
	username, password, err := mdt.server.GetAuthenticationCredentials()
	if err != nil {
		return err
	}
	remoteRepositoryFullPath, err := url.JoinPath(mdt.server.ArtifactoryUrl, mdt.depsRepo)
	if err != nil {
		return err
	}
	mdt.settingsXmlPath = filepath.Join(path, settingsXmlFile)
	settingsXmlContent := fmt.Sprintf(settingsXmlTemplate, username, password, remoteRepositoryFullPath)
	return errorutils.CheckError(os.WriteFile(mdt.settingsXmlPath, []byte(settingsXmlContent), 0666))
}
