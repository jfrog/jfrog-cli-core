package npm

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/buger/jsonparser"
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/artifactory/utils/npm"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-cli-core/utils/ioutils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	serviceutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/utils/version"
)

const npmrcFileName = ".npmrc"
const npmrcBackupFileName = "jfrog.npmrc.backup"
const minSupportedArtifactoryVersion = "5.5.2"
const minSupportedNpmVersion = "5.4.0"

type NpmCommandArgs struct {
	command          string
	threads          int
	jsonOutput       bool
	executablePath   string
	npmrcFileMode    os.FileMode
	workingDirectory string
	registry         string
	npmAuth          string
	collectBuildInfo bool
	dependencies     map[string]*dependency
	typeRestriction  string
	authArtDetails   auth.ServiceDetails
	packageInfo      *npm.PackageInfo
	NpmCommand
}

type NpmInstallOrCiCommand struct {
	configFilePath      string
	internalCommandName string
	*NpmCommandArgs
}

func NewNpmInstallCommand() *NpmInstallOrCiCommand {
	return &NpmInstallOrCiCommand{NpmCommandArgs: NewNpmCommandArgs("install"), internalCommandName: "rt_npm_install"}
}

func NewNpmCiCommand() *NpmInstallOrCiCommand {
	return &NpmInstallOrCiCommand{NpmCommandArgs: NewNpmCommandArgs("ci"), internalCommandName: "rt_npm_ci"}
}

func (nic *NpmInstallOrCiCommand) CommandName() string {
	return nic.internalCommandName
}

func (nic *NpmInstallOrCiCommand) SetConfigFilePath(configFilePath string) *NpmInstallOrCiCommand {
	nic.configFilePath = configFilePath
	return nic
}

func (nic *NpmInstallOrCiCommand) SetArgs(args []string) *NpmInstallOrCiCommand {
	nic.NpmCommandArgs.npmArgs = args
	return nic
}

func (nic *NpmInstallOrCiCommand) SetRepoConfig(conf *utils.RepositoryConfig) *NpmInstallOrCiCommand {
	serverDetails, _ := conf.ServerDetails()
	nic.NpmCommandArgs.SetRepo(conf.TargetRepo()).SetServerDetails(serverDetails)
	return nic
}

func (nic *NpmInstallOrCiCommand) Run() error {
	log.Info(fmt.Sprintf("Running npm %s.", nic.command))
	// Read config file.
	log.Debug("Preparing to read the config file", nic.configFilePath)
	vConfig, err := utils.ReadConfigFile(nic.configFilePath, utils.YAML)
	if err != nil {
		return err
	}
	// Extract resolution params.
	resolverParams, err := utils.GetRepoConfigByPrefix(nic.configFilePath, utils.ProjectConfigResolverPrefix, vConfig)
	if err != nil {
		return err
	}
	threads, jsonOutput, filteredNpmArgs, buildConfiguration, err := npm.ExtractNpmOptionsFromArgs(nic.npmArgs)
	nic.SetRepoConfig(resolverParams).SetArgs(filteredNpmArgs).SetThreads(threads).SetJsonOutput(jsonOutput).SetBuildConfiguration(buildConfiguration)
	if err != nil {
		return err
	}
	return nic.run()
}

func (nca *NpmCommandArgs) SetThreads(threads int) *NpmCommandArgs {
	nca.threads = threads
	return nca
}

func (nca *NpmCommandArgs) SetJsonOutput(jsonOutput bool) *NpmCommandArgs {
	nca.jsonOutput = jsonOutput
	return nca
}

func NewNpmCommandArgs(npmCommand string) *NpmCommandArgs {
	return &NpmCommandArgs{command: npmCommand}
}

func (nca *NpmCommandArgs) ServerDetails() (*config.ServerDetails, error) {
	return nca.serverDetails, nil
}

func (nca *NpmCommandArgs) run() error {
	if err := nca.preparePrerequisites(nca.repo); err != nil {
		return err
	}

	if err := nca.createTempNpmrc(); err != nil {
		return nca.restoreNpmrcAndError(err)
	}

	if err := nca.runInstallOrCi(); err != nil {
		return nca.restoreNpmrcAndError(err)
	}

	if err := nca.restoreNpmrc(); err != nil {
		return err
	}

	if !nca.collectBuildInfo {
		log.Info(fmt.Sprintf("npm %s finished successfully.", nca.command))
		return nil
	}

	if err := nca.setDependenciesList(); err != nil {
		return err
	}

	if err := nca.collectDependenciesChecksums(); err != nil {
		return err
	}

	if err := nca.saveDependenciesData(); err != nil {
		return err
	}

	log.Info(fmt.Sprintf("npm %s finished successfully.", nca.command))
	return nil
}

func (nca *NpmCommandArgs) preparePrerequisites(repo string) error {
	log.Debug("Preparing prerequisites.")
	if err := nca.setNpmExecutable(); err != nil {
		return err
	}

	if err := nca.validateNpmVersion(); err != nil {
		return err
	}

	if err := nca.setWorkingDirectory(); err != nil {
		return err
	}

	if err := nca.prepareArtifactoryPrerequisites(repo); err != nil {
		return err
	}

	if err := nca.prepareBuildInfo(); err != nil {
		return err
	}

	return nca.backupProjectNpmrc()
}

func (nca *NpmCommandArgs) prepareArtifactoryPrerequisites(repo string) (err error) {
	npmAuth, err := getArtifactoryDetails(nca.authArtDetails)
	if err != nil {
		return err
	}
	nca.npmAuth = npmAuth

	if err = utils.CheckIfRepoExists(repo, nca.authArtDetails); err != nil {
		return err
	}

	nca.registry = getNpmRepositoryUrl(repo, nca.authArtDetails.GetUrl())
	return nil
}

func (nca *NpmCommandArgs) prepareBuildInfo() error {
	var err error
	if len(nca.buildConfiguration.BuildName) > 0 && len(nca.buildConfiguration.BuildNumber) > 0 {
		nca.collectBuildInfo = true
		if err = utils.SaveBuildGeneralDetails(nca.buildConfiguration.BuildName, nca.buildConfiguration.BuildNumber); err != nil {
			return err
		}

		if nca.packageInfo, err = npm.ReadPackageInfoFromPackageJson(nca.workingDirectory); err != nil {
			return err
		}
	}
	return err
}

func (nca *NpmCommandArgs) setWorkingDirectory() error {
	currentDir, err := os.Getwd()
	if err != nil {
		return errorutils.CheckError(err)
	}

	if currentDir, err = filepath.Abs(currentDir); err != nil {
		return errorutils.CheckError(err)
	}

	nca.workingDirectory = currentDir
	log.Debug("Working directory set to:", nca.workingDirectory)
	if err = nca.setArtifactoryAuth(); err != nil {
		return errorutils.CheckError(err)
	}
	return nil
}

// In order to make sure the install/ci downloads the dependencies from Artifactory, we are creating a.npmrc file in the project's root directory.
// If such a file already exists, we are copying it aside.
// This method restores the backed up file and deletes the one created by the command.
func (nca *NpmCommandArgs) restoreNpmrc() (err error) {
	log.Debug("Restoring project .npmrc file")
	if err = os.Remove(filepath.Join(nca.workingDirectory, npmrcFileName)); err != nil {
		return errorutils.CheckError(errors.New(createRestoreErrorPrefix(nca.workingDirectory) + err.Error()))
	}
	log.Debug("Deleted the temporary .npmrc file successfully")

	if _, err = os.Stat(filepath.Join(nca.workingDirectory, npmrcBackupFileName)); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errorutils.CheckError(errors.New(createRestoreErrorPrefix(nca.workingDirectory) + err.Error()))
	}

	if err = ioutils.CopyFile(
		filepath.Join(nca.workingDirectory, npmrcBackupFileName),
		filepath.Join(nca.workingDirectory, npmrcFileName), nca.npmrcFileMode); err != nil {
		return errorutils.CheckError(err)
	}
	log.Debug("Restored project .npmrc file successfully")

	if err = os.Remove(filepath.Join(nca.workingDirectory, npmrcBackupFileName)); err != nil {
		return errorutils.CheckError(errors.New(createRestoreErrorPrefix(nca.workingDirectory) + err.Error()))
	}
	log.Debug("Deleted project", npmrcBackupFileName, "file successfully")
	return nil
}

func createRestoreErrorPrefix(workingDirectory string) string {
	return fmt.Sprintf("Error occurred while restoring project .npmrc file. "+
		"Delete '%s' and move '%s' (if exists) to '%s' in order to restore the project. Failure cause: \n",
		filepath.Join(workingDirectory, npmrcFileName),
		filepath.Join(workingDirectory, npmrcBackupFileName),
		filepath.Join(workingDirectory, npmrcFileName))
}

// In order to make sure the install/ci downloads the artifacts from Artifactory we create a .npmrc file in the project dir.
// If such a file exists we back it up as npmrcBackupFileName.
func (nca *NpmCommandArgs) createTempNpmrc() error {
	log.Debug("Creating project .npmrc file.")
	data, err := npm.GetConfigList(nca.npmArgs, nca.executablePath)
	configData, err := nca.prepareConfigData(data)
	if err != nil {
		return errorutils.CheckError(err)
	}

	if err = removeNpmrcIfExists(nca.workingDirectory); err != nil {
		return err
	}

	return errorutils.CheckError(ioutil.WriteFile(filepath.Join(nca.workingDirectory, npmrcFileName), configData, nca.npmrcFileMode))
}

func (nca *NpmCommandArgs) runInstallOrCi() error {
	log.Debug(fmt.Sprintf("Running npm %s command.", nca.command))
	filteredArgs := filterFlags(nca.npmArgs)
	npmCmdConfig := &npm.NpmConfig{
		Npm:          nca.executablePath,
		Command:      append([]string{nca.command}, filteredArgs...),
		CommandFlags: nil,
		StrWriter:    nil,
		ErrWriter:    nil,
	}

	if nca.collectBuildInfo && len(filteredArgs) > 0 {
		log.Warn("Build info dependencies collection with npm arguments is not supported. Build info creation will be skipped.")
		nca.collectBuildInfo = false
	}

	return errorutils.CheckError(gofrogcmd.RunCmd(npmCmdConfig))
}

func (nca *NpmCommandArgs) setDependenciesList() (err error) {
	nca.dependencies = make(map[string]*dependency)
	// nca.scope can be empty, "production" or "development" in case of empty both of the functions should run
	if nca.typeRestriction != "production" {
		if err = nca.prepareDependencies("development"); err != nil {
			return
		}
	}
	if nca.typeRestriction != "development" {
		err = nca.prepareDependencies("production")
	}
	return
}

func (nca *NpmCommandArgs) collectDependenciesChecksums() error {
	log.Info("Collecting dependencies information... This may take a few minutes...")
	servicesManager, err := utils.CreateServiceManager(nca.serverDetails, false)
	if err != nil {
		return err
	}

	previousBuildDependencies, err := getDependenciesFromLatestBuild(servicesManager, nca.buildConfiguration.BuildName)
	if err != nil {
		return err
	}
	producerConsumer := parallel.NewBounedRunner(nca.threads, false)
	errorsQueue := clientutils.NewErrorsQueue(1)
	handlerFunc := nca.createGetDependencyInfoFunc(servicesManager, previousBuildDependencies)
	go func() {
		defer producerConsumer.Done()
		for i := range nca.dependencies {
			producerConsumer.AddTaskWithError(handlerFunc(i), errorsQueue.AddError)
		}
	}()
	producerConsumer.Run()
	return errorsQueue.GetError()
}

func getDependenciesFromLatestBuild(servicesManager artifactory.ArtifactoryServicesManager, buildName string) (map[string]*buildinfo.Dependency, error) {
	buildDependencies := make(map[string]*buildinfo.Dependency)
	previousBuild, found, err := servicesManager.GetBuildInfo(services.BuildInfoParams{BuildName: buildName, BuildNumber: "LATEST"})
	if err != nil || !found {
		return buildDependencies, err
	}
	for _, module := range previousBuild.BuildInfo.Modules {
		for _, dependency := range module.Dependencies {
			buildDependencies[dependency.Id] = &buildinfo.Dependency{Id: dependency.Id,
				Checksum: &buildinfo.Checksum{Md5: dependency.Md5, Sha1: dependency.Sha1}}
		}
	}
	return buildDependencies, nil
}

func (nca *NpmCommandArgs) saveDependenciesData() error {
	log.Debug("Saving data.")
	dependencies, missingDependencies := nca.transformDependencies()
	populateFunc := func(partial *buildinfo.Partial) {
		partial.Dependencies = dependencies
		if nca.buildConfiguration.Module == "" {
			nca.buildConfiguration.Module = nca.packageInfo.BuildInfoModuleId()
		}
		partial.ModuleId = nca.buildConfiguration.Module
		partial.ModuleType = buildinfo.Npm
	}

	if err := utils.SavePartialBuildInfo(nca.buildConfiguration.BuildName, nca.buildConfiguration.BuildNumber, populateFunc); err != nil {
		return err
	}

	if len(missingDependencies) > 0 {
		var missingDependenciesText []string
		for _, dependency := range missingDependencies {
			missingDependenciesText = append(missingDependenciesText, dependency.name+":"+dependency.version)
		}
		log.Warn(strings.Join(missingDependenciesText, "\n"))
		log.Warn("The npm dependencies above could not be found in Artifactory and therefore are not included in the build-info.\n" +
			"Make sure the dependencies are available in Artifactory for this build.\n" +
			"Deleting the local cache will force populating Artifactory with these dependencies.")
	}
	return nil
}

func (nca *NpmCommandArgs) validateNpmVersion() error {
	npmVersion, err := npm.Version(nca.executablePath)
	if err != nil {
		return err
	}
	rtVersion := version.NewVersion(string(npmVersion))
	if rtVersion.Compare(minSupportedNpmVersion) > 0 {
		return errorutils.CheckError(errors.New(fmt.Sprintf(
			"JFrog CLI npm %s command requires npm client version "+minSupportedNpmVersion+" or higher", nca.command)))
	}
	return nil
}

// To make npm do the resolution from Artifactory we are creating .npmrc file in the project dir.
// If a .npmrc file already exists we will backup it and override while running the command
func (nca *NpmCommandArgs) backupProjectNpmrc() error {
	fileInfo, err := os.Stat(filepath.Join(nca.workingDirectory, npmrcFileName))
	if err != nil {
		if os.IsNotExist(err) {
			nca.npmrcFileMode = 0644
			return nil
		}
		return errorutils.CheckError(err)
	}

	nca.npmrcFileMode = fileInfo.Mode()
	src := filepath.Join(nca.workingDirectory, npmrcFileName)
	dst := filepath.Join(nca.workingDirectory, npmrcBackupFileName)
	if err = ioutils.CopyFile(src, dst, nca.npmrcFileMode); err != nil {
		return err
	}
	log.Debug("Project .npmrc file backed up successfully to", filepath.Join(nca.workingDirectory, npmrcBackupFileName))
	return nil
}

// This func transforms "npm config list --json" result to key=val list of values that can be set to .npmrc file.
// it filters any nil values key, changes registry and scope registries to Artifactory url and adds Artifactory authentication to the list
func (nca *NpmCommandArgs) prepareConfigData(data []byte) ([]byte, error) {
	var collectedConfig map[string]interface{}
	var filteredConf []string
	if err := json.Unmarshal(data, &collectedConfig); err != nil {
		return nil, errorutils.CheckError(err)
	}

	for i := range collectedConfig {
		if isValidKeyVal(i, collectedConfig[i]) {
			filteredConf = append(filteredConf, i, " = ", fmt.Sprint(collectedConfig[i]), "\n")
		} else if strings.HasPrefix(i, "@") {
			// Override scoped registries (@scope = xyz)
			filteredConf = append(filteredConf, i, " = ", nca.registry, "\n")
		}
		nca.setTypeRestriction(i, collectedConfig[i])
	}
	filteredConf = append(filteredConf, "json = ", strconv.FormatBool(nca.jsonOutput), "\n")
	filteredConf = append(filteredConf, "registry = ", nca.registry, "\n")
	filteredConf = append(filteredConf, nca.npmAuth)
	return []byte(strings.Join(filteredConf, "")), nil
}

// npm install/ci type restriction can be set by "--production" or "-only={prod[uction]|dev[elopment]}" flags
func (nca *NpmCommandArgs) setTypeRestriction(key string, val interface{}) {
	if key == "production" && val != nil && (val == true || val == "true") {
		nca.typeRestriction = "production"
	} else if key == "only" && val != nil {
		if strings.Contains(val.(string), "prod") {
			nca.typeRestriction = "production"
		} else if strings.Contains(val.(string), "dev") {
			nca.typeRestriction = "development"
		}
	}
}

// Run npm list and parse the returned json
func (nca *NpmCommandArgs) prepareDependencies(typeRestriction string) error {
	// Run npm list
	data, errData, err := npm.RunList(strings.Join(append(nca.npmArgs, " -only="+typeRestriction), " "), nca.executablePath)
	if err != nil {
		log.Warn("npm list command failed with error:", err.Error())
	}
	if len(errData) > 0 {
		log.Warn("Some errors occurred while collecting dependencies info:\n" + string(errData))
	}

	// Parse the dependencies json object
	return jsonparser.ObjectEach(data, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
		if string(key) == "dependencies" {
			err := nca.parseDependencies(value, typeRestriction)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// Parses npm dependencies recursively and adds the collected dependencies to nca.dependencies
func (nca *NpmCommandArgs) parseDependencies(data []byte, scope string) error {
	var transitiveDependencies [][]byte
	err := jsonparser.ObjectEach(data, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
		ver, _, _, err := jsonparser.Get(data, string(key), "version")
		if err != nil && err != jsonparser.KeyPathNotFoundError {
			return errorutils.CheckError(err)
		} else if err == jsonparser.KeyPathNotFoundError {
			log.Warn(fmt.Sprintf("npm dependencies list contains the package '%s' without version information. The dependency will not be added to build-info.", string(key)))
		} else {
			nca.appendDependency(key, ver, scope)
		}
		transitive, _, _, err := jsonparser.Get(data, string(key), "dependencies")
		if err != nil && err.Error() != "Key path not found" {
			return errorutils.CheckError(err)
		}

		if len(transitive) > 0 {
			transitiveDependencies = append(transitiveDependencies, transitive)
		}
		return nil
	})

	if err != nil {
		return err
	}

	for _, element := range transitiveDependencies {
		err := nca.parseDependencies(element, scope)
		if err != nil {
			return err
		}
	}
	return nil
}

func (nca *NpmCommandArgs) appendDependency(key []byte, ver []byte, scope string) {
	dependencyKey := string(key) + ":" + string(ver)
	if nca.dependencies[dependencyKey] == nil {
		nca.dependencies[dependencyKey] = &dependency{name: string(key), version: string(ver), scopes: []string{scope}}
	} else if !scopeAlreadyExists(scope, nca.dependencies[dependencyKey].scopes) {
		nca.dependencies[dependencyKey].scopes = append(nca.dependencies[dependencyKey].scopes, scope)
	}
}

// Creates a function that fetches dependency data.
// If a dependency was included in the previous build, take the checksums information from it.
// Otherwise, fetch the checksum from Artifactory.
// Can be applied from a producer-consumer mechanism.
func (nca *NpmCommandArgs) createGetDependencyInfoFunc(servicesManager artifactory.ArtifactoryServicesManager,
	previousBuildDependencies map[string]*buildinfo.Dependency) getDependencyInfoFunc {
	return func(dependencyIndex string) parallel.TaskFunc {
		return func(threadId int) error {
			name := nca.dependencies[dependencyIndex].name
			ver := nca.dependencies[dependencyIndex].version

			// Get dependency info.
			checksum, fileType, err := getDependencyInfo(name, ver, previousBuildDependencies, servicesManager, threadId)
			if err != nil || checksum == nil {
				return err
			}

			// Update dependency.
			nca.dependencies[dependencyIndex].fileType = fileType
			nca.dependencies[dependencyIndex].checksum = checksum
			return nil
		}
	}
}

// Get dependency's checksum and type.
func getDependencyInfo(name, ver string, previousBuildDependencies map[string]*buildinfo.Dependency,
	servicesManager artifactory.ArtifactoryServicesManager, threadId int) (checksum *buildinfo.Checksum, fileType string, err error) {
	id := name + ":" + ver
	if dep, ok := previousBuildDependencies[id]; ok {
		// Get checksum from previous build.
		checksum = dep.Checksum
		fileType = dep.Type
		return
	}

	// Get info from Artifactory.
	log.Debug(clientutils.GetLogMsgPrefix(threadId, false), "Fetching checksums for", name, ":", ver)
	var stream io.ReadCloser
	stream, err = servicesManager.Aql(serviceutils.CreateAqlQueryForNpm(name, ver))
	if err != nil {
		return
	}
	defer stream.Close()
	var result []byte
	result, err = ioutil.ReadAll(stream)
	if err != nil {
		return
	}
	parsedResult := new(aqlResult)
	if err = json.Unmarshal(result, parsedResult); err != nil {
		return nil, "", errorutils.CheckError(err)
	}
	if len(parsedResult.Results) == 0 {
		log.Debug(clientutils.GetLogMsgPrefix(threadId, false), name, ":", ver, "could not be found in Artifactory.")
		return
	}
	if i := strings.LastIndex(parsedResult.Results[0].Name, "."); i != -1 {
		fileType = parsedResult.Results[0].Name[i+1:]
	}
	log.Debug(clientutils.GetLogMsgPrefix(threadId, false), "Found", parsedResult.Results[0].Name,
		"sha1:", parsedResult.Results[0].Actual_sha1,
		"md5", parsedResult.Results[0].Actual_md5)

	checksum = &buildinfo.Checksum{Sha1: parsedResult.Results[0].Actual_sha1, Md5: parsedResult.Results[0].Actual_md5}
	return
}

// Transforms the list of dependencies to buildinfo.Dependencies list and creates a list of dependencies that are missing in Artifactory.
func (nca *NpmCommandArgs) transformDependencies() (dependencies []buildinfo.Dependency, missingDependencies []dependency) {
	for _, dependency := range nca.dependencies {
		if dependency.checksum != nil {
			dependencies = append(dependencies,
				buildinfo.Dependency{Id: dependency.name + ":" + dependency.version, Type: dependency.fileType,
					Scopes: dependency.scopes, Checksum: dependency.checksum})
		} else {
			missingDependencies = append(missingDependencies, *dependency)
		}
	}
	return
}

func (nca *NpmCommandArgs) restoreNpmrcAndError(err error) error {
	if restoreErr := nca.restoreNpmrc(); restoreErr != nil {
		return errors.New(fmt.Sprintf("Two errors occurred:\n %s\n %s", restoreErr.Error(), err.Error()))
	}
	return err
}

func (nca *NpmCommandArgs) setArtifactoryAuth() error {
	authArtDetails, err := nca.serverDetails.CreateArtAuthConfig()
	if err != nil {
		return err
	}
	if authArtDetails.GetSshAuthHeaders() != nil {
		return errorutils.CheckError(errors.New("SSH authentication is not supported in this command"))
	}
	nca.authArtDetails = authArtDetails
	return nil
}

func removeNpmrcIfExists(workingDirectory string) error {
	if _, err := os.Stat(filepath.Join(workingDirectory, npmrcFileName)); err != nil {
		if os.IsNotExist(err) { // The file dose not exist, nothing to do.
			return nil
		}
		return errorutils.CheckError(err)
	}

	log.Debug("Removing Existing .npmrc file")
	return errorutils.CheckError(os.Remove(filepath.Join(workingDirectory, npmrcFileName)))
}

func (nca *NpmCommandArgs) setNpmExecutable() error {
	npmExecPath, err := exec.LookPath("npm")
	if err != nil {
		return errorutils.CheckError(err)
	}

	if npmExecPath == "" {
		return errorutils.CheckError(errors.New("could not find 'npm' executable"))
	}
	nca.executablePath = npmExecPath
	log.Debug("Found npm executable at:", nca.executablePath)
	return nil
}

func getArtifactoryDetails(authArtDetails auth.ServiceDetails) (npmAuth string, err error) {
	// Check Artifactory version.
	err = validateArtifactoryVersion(authArtDetails)
	if err != nil {
		return "", err
	}

	// Get npm token from Artifactory.
	if authArtDetails.GetAccessToken() == "" {
		return getDetailsUsingBasicAuth(authArtDetails)
	}
	return getDetailsUsingAccessToken(authArtDetails)
}

func validateArtifactoryVersion(artDetails auth.ServiceDetails) error {
	// Get Artifactory version.
	versionStr, err := artDetails.GetVersion()
	if err != nil {
		return err
	}

	// Validate version.
	rtVersion := version.NewVersion(versionStr)
	if !rtVersion.AtLeast(minSupportedArtifactoryVersion) {
		return errorutils.CheckError(errors.New("this operation requires Artifactory version " + minSupportedArtifactoryVersion + " or higher"))
	}

	return nil
}

func getDetailsUsingAccessToken(artDetails auth.ServiceDetails) (npmAuth string, err error) {
	npmAuthString := "_auth = %s\nalways-auth = true"
	// Build npm token, consists of <username:password> encoded.
	// Use Artifactory's access-token as username and password to create npm token.
	username, err := auth.ExtractUsernameFromAccessToken(artDetails.GetAccessToken())
	if err != nil {
		return "", err
	}
	encodedNpmToken := base64.StdEncoding.EncodeToString([]byte(username + ":" + artDetails.GetAccessToken()))
	npmAuth = fmt.Sprintf(npmAuthString, encodedNpmToken)

	return npmAuth, err
}

func getDetailsUsingBasicAuth(artDetails auth.ServiceDetails) (npmAuth string, err error) {
	authApiUrl := artDetails.GetUrl() + "api/npm/auth"
	log.Debug("Sending npm auth request")

	// Get npm token from Artifactory.
	client, err := httpclient.ClientBuilder().Build()
	if err != nil {
		return "", err
	}
	resp, body, _, err := client.SendGet(authApiUrl, true, artDetails.CreateHttpClientDetails())
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", errorutils.CheckError(errors.New("Artifactory response: " + resp.Status + "\n" + clientutils.IndentJson(body)))
	}

	return string(body), nil
}

func getNpmRepositoryUrl(repo, url string) string {
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	url += "api/npm/" + repo
	return url
}

func scopeAlreadyExists(scope string, existingScopes []string) bool {
	for _, existingScope := range existingScopes {
		if existingScope == scope {
			return true
		}
	}
	return false
}

// Valid configs keys are not related to registry (registry = xyz) or scoped registry (@scope = xyz)) and have data in their value
// We want to avoid writing "json=true" because we ran the the configuration list command with "--json". We will add it explicitly if necessary.
func isValidKeyVal(key string, val interface{}) bool {
	return !strings.HasPrefix(key, "//") &&
		!strings.HasPrefix(key, "@") &&
		key != "registry" &&
		key != "metrics-registry" &&
		key != "json" &&
		val != nil &&
		val != ""
}

func filterFlags(splitArgs []string) []string {
	var filteredArgs []string
	for _, arg := range splitArgs {
		if !strings.HasPrefix(arg, "-") {
			filteredArgs = append(filteredArgs, arg)
		}
	}
	return filteredArgs
}

type getDependencyInfoFunc func(string) parallel.TaskFunc

type dependency struct {
	name     string
	version  string
	scopes   []string
	fileType string
	checksum *buildinfo.Checksum
}

type aqlResult struct {
	Results []*results `json:"results,omitempty"`
}

type results struct {
	Name        string `json:"name,omitempty"`
	Actual_md5  string `json:"actual_md5,omitempty"`
	Actual_sha1 string `json:"actual_sha1,omitempty"`
}
