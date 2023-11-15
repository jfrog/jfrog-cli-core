package java

import (
	_ "embed"
	"encoding/base64"
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
	basicAuthServerXmlPath = "resources/basic-auth-server.xml"
	tokenAuthServerXmlPath = "resources/token-auth-server.xml"
	errReadServerXml       = "encountered an error while attempting to read from %s while constructing the settings.xml for the 'mvn-dep-tree' command:\n%w"
)

//go:embed resources/settings.xml
var settingsXmlTemplate string

//go:embed resources/maven-dep-tree.jar
var mavenDepTreeJar []byte

type MavenDepTreeManager struct {
	*DepTreeManager
	isInstalled     bool
	cmdName         string
	settingsXmlPath string
}

func NewMavenDepTreeManager(params *DepTreeParams, cmdName string, isDepTreeInstalled bool) *MavenDepTreeManager {
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
	manager := NewMavenDepTreeManager(params, TreeCmd, isDepTreeInstalled)
	outputFileContent, err := manager.RunMavenDepTree()
	if err != nil {
		return
	}
	dependencyTree, uniqueDeps, err = getGraphFromDepTree(outputFileContent)
	return
}

func (mdt *MavenDepTreeManager) RunMavenDepTree() ([]byte, error) {
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
	_, err := mdt.RunMvnCmd(goals)
	return err
}

func GetMavenPluginInstallationGoals(pluginPath string) []string {
	return []string{"org.apache.maven.plugins:maven-install-plugin:3.1.1:install-file", "-Dfile=" + pluginPath}
}

func (mdt *MavenDepTreeManager) execMavenDepTree(depTreeExecDir string) ([]byte, error) {
	if mdt.cmdName == TreeCmd {
		return mdt.runTreeCmd(depTreeExecDir)
	}
	return mdt.runProjectsCmd()
}

func (mdt *MavenDepTreeManager) runTreeCmd(depTreeExecDir string) ([]byte, error) {
	mavenDepTreePath := filepath.Join(depTreeExecDir, mavenDepTreeOutputFile)
	goals := []string{"com.jfrog:maven-dep-tree:" + mavenDepTreeVersion + ":" + TreeCmd, "-DdepsTreeOutputFile=" + mavenDepTreePath}
	if _, err := mdt.RunMvnCmd(goals); err != nil {
		return nil, err
	}

	mavenDepTreeOutput, err := os.ReadFile(mavenDepTreePath)
	err = errorutils.CheckError(err)
	return mavenDepTreeOutput, err
}

func (mdt *MavenDepTreeManager) runProjectsCmd() ([]byte, error) {
	goals := []string{"com.jfrog:maven-dep-tree:" + mavenDepTreeVersion + ":" + ProjectsCmd, "-q"}
	return mdt.RunMvnCmd(goals)
}

func (mdt *MavenDepTreeManager) RunMvnCmd(goals []string) ([]byte, error) {
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
	return cmdOutput, err
}

// Creates a new settings.xml file configured with the provided server and repository from the current MavenDepTreeManager instance.
// The settings.xml will be written to the given path.
func (mdt *MavenDepTreeManager) createSettingsXmlWithConfiguredArtifactory(path string) error {
	serverAuthentication, err := mdt.getSettingsXmlServerAuthentication()
	if err != nil {
		return err
	}
	remoteRepositoryFullPath, err := url.JoinPath(mdt.server.ArtifactoryUrl, mdt.depsRepo)
	if err != nil {
		return err
	}
	mdt.settingsXmlPath = filepath.Join(path, settingsXmlFile)
	settingsXmlContent := fmt.Sprintf(settingsXmlTemplate, serverAuthentication, remoteRepositoryFullPath)
	return errorutils.CheckError(os.WriteFile(mdt.settingsXmlPath, []byte(settingsXmlContent), 0600))
}

func (mdt *MavenDepTreeManager) getSettingsXmlServerAuthentication() (string, error) {
	username, password, token := mdt.server.User, mdt.server.Password, mdt.server.AccessToken
	if username != "" {
		if password == "" {
			password = token
		}
		basicAuthServerXml, err := os.ReadFile(basicAuthServerXmlPath)
		if err != nil {
			return "", errorutils.CheckErrorf(errReadServerXml, basicAuthServerXmlPath, err)
		}
		authString := fmt.Sprintf(string(basicAuthServerXml), base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
		return authString, nil
	}

	if token == "" {
		errorMessage := "both a username and either a password or an access token are required when configuring the Maven settings.xml file to execute Maven commands for resolving dependencies from Artifactory"
		return "", errorutils.CheckErrorf(errorMessage)
	}

	tokenAuthServerXml, err := os.ReadFile(tokenAuthServerXmlPath)
	if err != nil {
		return "", errorutils.CheckErrorf(errReadServerXml, tokenAuthServerXmlPath, err)
	}
	authString := fmt.Sprintf(string(tokenAuthServerXml), token)
	return authString, nil
}
