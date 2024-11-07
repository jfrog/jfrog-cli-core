package golang

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jfrog/build-info-go/build"
	biutils "github.com/jfrog/build-info-go/utils"
	buildUtils "github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-cli-core/v2/common/project"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	rtutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type GoCommand struct {
	goArg              []string
	buildConfiguration *buildUtils.BuildConfiguration
	deployerParams     *project.RepositoryConfig
	resolverParams     *project.RepositoryConfig
	configFilePath     string
	noFallback         bool
}

func NewGoCommand() *GoCommand {
	return &GoCommand{}
}

func (gc *GoCommand) SetConfigFilePath(configFilePath string) *GoCommand {
	gc.configFilePath = configFilePath
	return gc
}

func (gc *GoCommand) SetResolverParams(resolverParams *project.RepositoryConfig) *GoCommand {
	gc.resolverParams = resolverParams
	return gc
}

func (gc *GoCommand) SetDeployerParams(deployerParams *project.RepositoryConfig) *GoCommand {
	gc.deployerParams = deployerParams
	return gc
}

func (gc *GoCommand) SetBuildConfiguration(buildConfiguration *buildUtils.BuildConfiguration) *GoCommand {
	gc.buildConfiguration = buildConfiguration
	return gc
}

func (gc *GoCommand) SetGoArg(goArg []string) *GoCommand {
	gc.goArg = goArg
	return gc
}

func (gc *GoCommand) CommandName() string {
	return "rt_go"
}

func (gc *GoCommand) ServerDetails() (*config.ServerDetails, error) {
	// If deployer Artifactory details exists, returns it.
	if gc.deployerParams != nil && !gc.deployerParams.IsServerDetailsEmpty() {
		return gc.deployerParams.ServerDetails()
	}

	// If resolver Artifactory details exists, returns it.
	if gc.resolverParams != nil && !gc.resolverParams.IsServerDetailsEmpty() {
		return gc.resolverParams.ServerDetails()
	}

	vConfig, err := project.ReadConfigFile(gc.configFilePath, project.YAML)
	if err != nil {
		return nil, err
	}
	return buildUtils.GetServerDetails(vConfig)
}

func (gc *GoCommand) Run() error {
	// Read config file.
	log.Debug("Preparing to read the config file", gc.configFilePath)
	vConfig, err := project.ReadConfigFile(gc.configFilePath, project.YAML)
	if err != nil {
		return err
	}

	// Extract resolution params.
	gc.resolverParams, err = project.GetRepoConfigByPrefix(gc.configFilePath, project.ProjectConfigResolverPrefix, vConfig)
	if err != nil {
		return err
	}

	if vConfig.IsSet(project.ProjectConfigDeployerPrefix) {
		// Extract deployer params.
		gc.deployerParams, err = project.GetRepoConfigByPrefix(gc.configFilePath, project.ProjectConfigDeployerPrefix, vConfig)
		if err != nil {
			return err
		}
	}

	// Extract build info information from the args.
	gc.goArg, gc.buildConfiguration, err = buildUtils.ExtractBuildDetailsFromArgs(gc.goArg)
	if err != nil {
		return err
	}

	// Extract no-fallback flag from the args.
	gc.goArg, err = gc.extractNoFallbackFromArgs()
	if err != nil {
		return err
	}
	return gc.run()
}

func (gc *GoCommand) extractNoFallbackFromArgs() (cleanArgs []string, err error) {
	var flagIndex int
	cleanArgs = append([]string(nil), gc.goArg...)

	// Extract no-fallback boolean flag from the args.
	flagIndex, gc.noFallback, err = coreutils.FindBooleanFlag("--no-fallback", cleanArgs)
	if err != nil {
		return
	}

	coreutils.RemoveFlagFromCommand(&cleanArgs, flagIndex, flagIndex)
	return
}

func (gc *GoCommand) run() (err error) {
	err = logGoVersion()
	if err != nil {
		return
	}
	goBuildInfo, err := buildUtils.PrepareBuildPrerequisites(gc.buildConfiguration)
	if err != nil {
		return
	}
	defer func() {
		if goBuildInfo != nil && err != nil {
			err = errors.Join(err, goBuildInfo.Clean())
		}
	}()

	resolverDetails, err := gc.resolverParams.ServerDetails()
	if err != nil {
		return
	}
	// If noFallback=false, missing packages will be fetched directly from VCS
	repoUrl, err := GetArtifactoryRemoteRepoUrl(resolverDetails, gc.resolverParams.TargetRepo(), GoProxyUrlParams{Direct: !gc.noFallback})
	if err != nil {
		return
	}

	err = biutils.RunGo(gc.goArg, repoUrl)
	if errorutils.CheckError(err) != nil {
		err = coreutils.ConvertExitCodeError(err)
		return
	}

	if goBuildInfo != nil {
		// Need to collect build info
		tempDirPath := ""
		if isGoGetCommand := len(gc.goArg) > 0 && gc.goArg[0] == "get"; isGoGetCommand {
			if len(gc.goArg) < 2 {
				// Package name was not supplied. Invalid go get commend
				err = errorutils.CheckErrorf("Invalid get command. Package name is missing")
				return
			}
			tempDirPath, err = fileutils.CreateTempDir()
			if err != nil {
				return
			}
			// Cleanup the temp working directory at the end.
			defer func() {
				err = errors.Join(err, fileutils.RemoveTempDir(tempDirPath))
			}()
			var serverDetails auth.ServiceDetails
			serverDetails, err = resolverDetails.CreateArtAuthConfig()
			if err != nil {
				return
			}
			err = copyGoPackageFiles(tempDirPath, gc.goArg[1], gc.resolverParams.TargetRepo(), serverDetails)
			if err != nil {
				return
			}
		}
		var goModule *build.GoModule
		goModule, err = goBuildInfo.AddGoModule(tempDirPath)
		if errorutils.CheckError(err) != nil {
			return
		}
		if gc.buildConfiguration.GetModule() != "" {
			goModule.SetName(gc.buildConfiguration.GetModule())
		}
		err = errorutils.CheckError(goModule.CalcDependencies())
	}

	return
}

// copyGoPackageFiles copies the package files from the go mod cache directory to the given destPath.
// The path to those cache files is retrieved using the supplied package name and Artifactory details.
func copyGoPackageFiles(destPath, packageName, rtTargetRepo string, authArtDetails auth.ServiceDetails) error {
	packageFilesPath, err := getPackageFilePathFromArtifactory(packageName, rtTargetRepo, authArtDetails)
	if err != nil {
		return err
	}
	// Copy the entire content of the relevant Go pkg directory to the requested destination path.
	err = biutils.CopyDir(packageFilesPath, destPath, true, nil)
	if err != nil {
		return fmt.Errorf("couldn't find suitable package files: %s", packageFilesPath)
	}
	// Set permission recursively
	return coreutils.SetPermissionsRecursively(destPath, 0755)
}

// getPackageFilePathFromArtifactory returns a string that represents the package files cache path.
// In most cases the path to those cache files is retrieved using the supplied package name and Artifactory details.
// However, if the user asked for a specific version (package@vX.Y.Z) the unnecessary call to Artifactory is avoided.
func getPackageFilePathFromArtifactory(packageName, rtTargetRepo string, authArtDetails auth.ServiceDetails) (packageFilesPath string, err error) {
	var version string
	packageCachePath, err := biutils.GetGoModCachePath()
	if errorutils.CheckError(err) != nil {
		return
	}
	packageNameSplitted := strings.Split(packageName, "@")
	name := packageNameSplitted[0]
	// The case the user asks for a specific version
	if len(packageNameSplitted) == 2 && strings.HasPrefix(packageNameSplitted[1], "v") {
		version = packageNameSplitted[1]
	} else {
		branchName := ""
		// The case the user asks for a specific branch
		if len(packageNameSplitted) == 2 {
			branchName = packageNameSplitted[1]
		}
		packageVersionRequest := buildPackageVersionRequest(name, branchName)
		// Retrieve the package version using Artifactory
		version, err = getPackageVersion(rtTargetRepo, packageVersionRequest, authArtDetails)
		if err != nil {
			return
		}
	}
	packageFilesPath, err = getFileSystemPackagePath(packageCachePath, name, version)
	return

}

// getPackageVersion returns the matching version for the packageName string using the Artifactory details that are provided.
// PackageName string should be in the following format: <Package Path>/@V/<Requested Branch Name>.info OR latest.info
// For example the jfrog/jfrog-cli/@v/master.info packageName will return the corresponding canonical version (vX.Y.Z) string for the jfrog-cli master branch.
func getPackageVersion(repoName, packageName string, details auth.ServiceDetails) (string, error) {
	artifactoryApiUrl, err := rtutils.BuildUrl(details.GetUrl(), "api/go/"+repoName, make(map[string]string))
	if err != nil {
		return "", err
	}
	artHttpDetails := details.CreateHttpClientDetails()
	client, err := httpclient.ClientBuilder().Build()
	if err != nil {
		return "", err
	}
	artifactoryApiUrl = artifactoryApiUrl + "/" + packageName
	resp, body, _, err := client.SendGet(artifactoryApiUrl, true, artHttpDetails, "")
	if err != nil {
		return "", err
	}
	if err = errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK); err != nil {
		return "", err
	}
	// Extract version from response
	var version PackageVersionResponseContent
	if err = json.Unmarshal(body, &version); err != nil {
		return "", errorutils.CheckError(err)
	}
	return version.Version, nil
}

type PackageVersionResponseContent struct {
	Version string `json:"Version,omitempty"`
}

// getFileSystemPackagePath returns a string that represents the package files cache path.
// In some cases when the path isn't represented by the package name, instead the name represents a specific project's directory's path.
// In this case we will scan the path until we find the package directory.
// Example : When running 'go get github.com/golang/mock/mockgen@v1.4.1'
//   - "mockgen" is a directory inside "mock" package ("mockgen" doesn't contain "go.mod").
//   - go download and save the whole "mock" package in local cache under 'github.com/golang/mock@v1.4.1' -- >
//     "go get" downloads and saves the whole "mock" package in the local cache under 'github.com/golang/mock@v1.4.1'
func getFileSystemPackagePath(packageCachePath, name, version string) (string, error) {
	separator := string(filepath.Separator)
	// For Windows OS
	path := filepath.Clean(name)
	for path != "" {
		packagePath := filepath.Join(packageCachePath, path+"@"+version)
		exists, err := fileutils.IsDirExists(packagePath, false)
		if err != nil {
			return "", err
		}
		if exists {
			return packagePath, nil
		}
		// Remove path's last element and check again
		path, _ = filepath.Split(path)
		path = strings.TrimSuffix(path, separator)
	}
	return "", errors.New("Could not find package: " + name + " in: " + packageCachePath)
}

// buildPackageVersionRequest returns a string representing the version request to Artifactory.
// The resulted string is in the following format: "<Package Name>/@V/<Branch Name>.info".
// If a branch name is not given, the branch name will be replaced with the "latest" keyword.
// ("<Package Name>/@V/latest.info").
func buildPackageVersionRequest(name, branchName string) string {
	packageVersionRequest := path.Join(name, "@v")
	if branchName != "" {
		// A branch name was given by the user
		return path.Join(packageVersionRequest, branchName+".info")
	}
	// No version was given to "go get" command, so the latest version should be requested
	return path.Join(packageVersionRequest, "latest.info")
}

func SetArtifactoryAsResolutionServer(serverDetails *config.ServerDetails, depsRepo string, goProxyParams GoProxyUrlParams) (err error) {
	if err = setGoProxy(serverDetails, depsRepo, goProxyParams); err != nil {
		err = fmt.Errorf("failed while setting Artifactory as a dependencies resolution registry: %s", err.Error())
	}
	return
}

func setGoProxy(server *config.ServerDetails, remoteGoRepo string, goProxyParams GoProxyUrlParams) error {
	repoUrl, err := GetArtifactoryRemoteRepoUrl(server, remoteGoRepo, goProxyParams)
	if err != nil {
		return err
	}
	return os.Setenv("GOPROXY", repoUrl)
}

func SetGoModCache(cacheFolder string) error {
	return os.Setenv("GOMODCACHE", cacheFolder)
}

func logGoVersion() error {
	version, err := biutils.GetParsedGoVersion()
	if err != nil {
		return errorutils.CheckError(err)
	}
	log.Info("Using go:", version.GetVersion())
	return nil
}

type GoProxyUrlParams struct {
	// Fallback to retrieve the modules directly from the source if
	// the module failed to be retrieved from the proxy.
	// add |direct to the end of the url.
	// example: https://gocenter.io|direct
	Direct bool
	// The path from baseUrl to the standard Go repository path
	// URL structure: <baseUrl>/<EndpointPrefix>/api/go/<repoName>
	EndpointPrefix string
}

func (gdu *GoProxyUrlParams) BuildUrl(url *url.URL, repoName string) string {
	url.Path = path.Join(url.Path, gdu.EndpointPrefix, "api/go/", repoName)

	return gdu.addDirect(url.String())
}

func (gdu *GoProxyUrlParams) addDirect(url string) string {
	if gdu.Direct && !strings.HasSuffix(url, "|direct") {
		return url + "|direct"
	}
	return url
}

func GetArtifactoryRemoteRepoUrl(serverDetails *config.ServerDetails, repo string, goProxyParams GoProxyUrlParams) (string, error) {
	authServerDetails, err := serverDetails.CreateArtAuthConfig()
	if err != nil {
		return "", err
	}
	return getArtifactoryApiUrl(repo, authServerDetails, goProxyParams)
}

// Gets the URL of the specified repository Go API in Artifactory.
// The URL contains credentials (username and access token or password).
func getArtifactoryApiUrl(repoName string, details auth.ServiceDetails, goProxyParams GoProxyUrlParams) (string, error) {
	rtUrl, err := url.Parse(details.GetUrl())
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	username := details.GetUser()
	password := details.GetPassword()

	// Get credentials from access-token if exists.
	if details.GetAccessToken() != "" {
		log.Debug("Using proxy with access-token.")
		if username == "" {
			username = auth.ExtractUsernameFromAccessToken(details.GetAccessToken())
		}
		password = details.GetAccessToken()
	}
	if password != "" {
		rtUrl.User = url.UserPassword(username, password)
	}

	return goProxyParams.BuildUrl(rtUrl, repoName), nil
}

func GetModuleName(projectDir string) (string, error) {
	path, err := biutils.GetModuleNameByDir(projectDir, log.Logger)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return path, nil
}
