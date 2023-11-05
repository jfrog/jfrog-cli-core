package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/access"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"golang.org/x/exp/slices"
)

const (
	MinJFrogProjectsArtifactoryVersion = "7.0.0"
	defaultAdminUsername               = "admin"
	defaultAdminPassword               = "password"
)

type TransferConfigBase struct {
	SourceServerDetails      *config.ServerDetails
	TargetServerDetails      *config.ServerDetails
	SourceArtifactoryManager artifactory.ArtifactoryServicesManager
	TargetArtifactoryManager artifactory.ArtifactoryServicesManager
	SourceAccessManager      *access.AccessServicesManager
	TargetAccessManager      *access.AccessServicesManager
	IncludeReposPatterns     []string
	ExcludeReposPatterns     []string
	FederatedMembersRemoved  bool
}

func NewTransferConfigBase(sourceServer, targetServer *config.ServerDetails) *TransferConfigBase {
	return &TransferConfigBase{
		SourceServerDetails: sourceServer,
		TargetServerDetails: targetServer,
	}
}

func (tcb *TransferConfigBase) SetIncludeReposPatterns(includeReposPatterns []string) *TransferConfigBase {
	tcb.IncludeReposPatterns = includeReposPatterns
	return tcb
}

func (tcb *TransferConfigBase) SetExcludeReposPatterns(excludeReposPatterns []string) *TransferConfigBase {
	tcb.ExcludeReposPatterns = excludeReposPatterns
	return tcb
}

func (tcb *TransferConfigBase) GetRepoFilter() *utils.IncludeExcludeFilter {
	return &utils.IncludeExcludeFilter{
		IncludePatterns: tcb.IncludeReposPatterns,
		ExcludePatterns: tcb.ExcludeReposPatterns,
	}
}

func (tcb *TransferConfigBase) CreateServiceManagers(dryRun bool) (err error) {
	if tcb.SourceArtifactoryManager, err = utils.CreateServiceManager(tcb.SourceServerDetails, -1, 0, dryRun); err != nil {
		return
	}
	if tcb.TargetArtifactoryManager, err = utils.CreateServiceManager(tcb.TargetServerDetails, -1, 0, dryRun); err != nil {
		return
	}
	if tcb.SourceAccessManager, err = utils.CreateAccessServiceManager(tcb.SourceServerDetails, false); err != nil {
		return
	}
	if tcb.TargetAccessManager, err = utils.CreateAccessServiceManager(tcb.TargetServerDetails, false); err != nil {
		return
	}
	return
}

// Make sure that the server is configured with a valid admin Access Token.
// serverDetails - The server to check
// accessManager - Access Manager to run ping
func (tcb *TransferConfigBase) ValidateAccessServerConnection(serverDetails *config.ServerDetails, accessManager *access.AccessServicesManager) error {
	if serverDetails.Password != "" {
		return errorutils.CheckErrorf("it looks like you configured the '%[1]s' instance with username and password.\n"+
			"This command can be used with admin Access Token only.\n"+
			"Please use the 'jf c edit %[1]s' command to configure the Access Token, and then re-run the command", serverDetails.ServerId)
	}

	if _, err := accessManager.Ping(); err != nil {
		return errors.Join(err, fmt.Errorf("the '%[1]s' instance Access Token is not valid. Please provide a valid access token by running the 'jf c edit %[1]s'", serverDetails.ServerId))
	}
	return nil
}

// Make sure source and target Artifactory URLs are different.
func (tcb *TransferConfigBase) ValidateDifferentServers() error {
	// Avoid exporting and importing to the same server
	log.Info("Verifying source and target servers are different...")
	if tcb.SourceServerDetails.GetArtifactoryUrl() == tcb.TargetServerDetails.GetArtifactoryUrl() {
		return errorutils.CheckErrorf("The source and target Artifactory servers are identical, but should be different.")
	}

	return nil
}

// Create a map between the repository types to the list of repositories to transfer.
func (tcb *TransferConfigBase) GetSelectedRepositories() (map[utils.RepoType][]services.RepositoryDetails, error) {
	allTargetRepos, err := tcb.getAllTargetRepositories()
	if err != nil {
		return nil, err
	}

	result := make(map[utils.RepoType][]services.RepositoryDetails, len(utils.RepoTypes)+1)
	sourceRepos, err := tcb.SourceArtifactoryManager.GetAllRepositories()
	if err != nil {
		return nil, err
	}

	includeExcludeFilter := tcb.GetRepoFilter()
	for _, sourceRepo := range *sourceRepos {
		if shouldIncludeRepo, err := includeExcludeFilter.ShouldIncludeRepository(sourceRepo.Key); err != nil {
			return nil, err
		} else if shouldIncludeRepo {
			if allTargetRepos.Exists(sourceRepo.Key) {
				log.Info("Repository '" + sourceRepo.Key + "' already exists in the target Artifactory server. Skipping.")
				continue
			}
			repoType := utils.RepoTypeFromString(sourceRepo.Type)
			result[repoType] = append(result[repoType], sourceRepo)
		}
	}
	return result, nil
}

// Deactivate key encryption in Artifactory, to allow retrieving raw text values in the artifactory-config.xml or in a remote repository.
func (tcb *TransferConfigBase) DeactivateKeyEncryption() (reactivateKeyEncryption func() error, err error) {
	var wasEncrypted bool
	if wasEncrypted, err = tcb.SourceArtifactoryManager.DeactivateKeyEncryption(); err != nil {
		return func() error { return nil }, err
	}
	if !wasEncrypted {
		return func() error { return nil }, nil
	}
	return tcb.SourceArtifactoryManager.ActivateKeyEncryption, nil
}

// Transfer all repositories to the target Artifactory server
// reposToTransfer - Map between a repository type to the list of repository names
// remoteRepositories - Remote repositories params, we get the remote repository params in an earlier stage after decryption
func (tcb *TransferConfigBase) TransferRepositoriesToTarget(reposToTransfer map[utils.RepoType][]services.RepositoryDetails, remoteRepositories []interface{}) (err error) {
	// Transfer remote repositories
	for i, remoteRepositoryName := range reposToTransfer[utils.Remote] {
		if err = tcb.createRepositoryAndAssignToProject(remoteRepositories[i], remoteRepositoryName); err != nil {
			return
		}
	}
	// Transfer local, federated and unknown repositories.
	for _, repoType := range []utils.RepoType{utils.Local, utils.Federated, utils.Unknown} {
		if len(reposToTransfer[repoType]) == 0 {
			continue
		}

		if err = tcb.transferSpecificRepositoriesToTarget(reposToTransfer[repoType], repoType); err != nil {
			return
		}
	}
	if len(reposToTransfer[utils.Virtual]) == 0 {
		return
	}
	return tcb.transferVirtualRepositoriesToTarget(reposToTransfer[utils.Virtual])
}

// Get a set of all repositories in the target Artifactory server.
func (tcb *TransferConfigBase) getAllTargetRepositories() (*datastructures.Set[string], error) {
	targetRepos, err := tcb.TargetArtifactoryManager.GetAllRepositories()
	if err != nil {
		return nil, err
	}
	allTargetRepos := datastructures.MakeSet[string]()
	for _, targetRepo := range *targetRepos {
		allTargetRepos.Add(targetRepo.Key)
	}
	return allTargetRepos, nil
}

// Transfer local, federated, unknown, or virtual repositories
// reposToTransfer - Repositories names to transfer
// repoType - Repository type
func (tcb *TransferConfigBase) transferSpecificRepositoriesToTarget(reposToTransfer []services.RepositoryDetails, repoType utils.RepoType) (err error) {
	for _, repo := range reposToTransfer {
		var params interface{}
		if err = tcb.SourceArtifactoryManager.GetRepository(repo.Key, &params); err != nil {
			return
		}
		if repoType == utils.Federated {
			if params, err = tcb.removeFederatedMembers(params); err != nil {
				return
			}
		}
		if err = tcb.createRepositoryAndAssignToProject(params, repo); err != nil {
			return
		}
	}
	return
}

// Transfer virtual repositories
// reposToTransfer - Repositories names to transfer
func (tcb *TransferConfigBase) transferVirtualRepositoriesToTarget(reposToTransfer []services.RepositoryDetails) (err error) {
	allReposParams := make(map[string]interface{})
	var singleRepoParamsMap map[string]interface{}
	var singleRepoParams interface{}
	// Step 1 - Get and create all virtual repositories with the included repositories removed
	for _, repoToTransfer := range reposToTransfer {
		// Get repository params
		if err = tcb.SourceArtifactoryManager.GetRepository(repoToTransfer.Key, &singleRepoParams); err != nil {
			return
		}
		allReposParams[repoToTransfer.Key] = singleRepoParams
		singleRepoParamsMap, err = InterfaceToMap(singleRepoParams)
		if err != nil {
			return
		}

		// Create virtual repository without included repositories
		repositories := singleRepoParamsMap["repositories"]
		delete(singleRepoParamsMap, "repositories")
		if err = tcb.createRepositoryAndAssignToProject(singleRepoParamsMap, repoToTransfer); err != nil {
			return
		}

		// Restore included repositories to set them later on
		if repositories != nil {
			singleRepoParamsMap["repositories"] = repositories
		}
	}

	// Step 2 - Update all virtual repositories with the included repositories
	for repoKey, repoParams := range allReposParams {
		if err = tcb.TargetArtifactoryManager.UpdateRepositoryWithParams(repoParams, repoKey); err != nil {
			return
		}
	}
	return
}

// Get all remote repositories. This method must run after disabling Artifactory key encryption.
// remoteRepositoryNames - Remote repository names to transfer
func (tcb *TransferConfigBase) GetAllRemoteRepositories(remoteRepositoryNames []string) ([]interface{}, error) {
	remoteRepositories := make([]interface{}, 0, len(remoteRepositoryNames))
	for _, repoKey := range remoteRepositoryNames {
		var repoDetails interface{}
		if err := tcb.SourceArtifactoryManager.GetRepository(repoKey, &repoDetails); err != nil {
			return nil, err
		}
		remoteRepositories = append(remoteRepositories, repoDetails)
	}
	return remoteRepositories, nil
}

func CreateArtifactoryClientDetails(serviceManager artifactory.ArtifactoryServicesManager) (*httputils.HttpClientDetails, error) {
	config := serviceManager.GetConfig()
	if config == nil {
		return nil, errorutils.CheckErrorf("expected full config, but no configuration exists")
	}
	rtDetails := config.GetServiceDetails()
	if rtDetails == nil {
		return nil, errorutils.CheckErrorf("artifactory details not configured")
	}
	clientDetails := rtDetails.CreateHttpClientDetails()
	return &clientDetails, nil
}

// Check if there is a configured user using default credentials 'admin:password' by pinging Artifactory.
func (tcb *TransferConfigBase) IsDefaultCredentials() (bool, error) {
	// Check if admin is locked
	lockedUsers, err := tcb.SourceArtifactoryManager.GetLockedUsers()
	if err != nil {
		return false, err
	}
	if slices.Contains(lockedUsers, defaultAdminUsername) {
		return false, nil
	}

	// Ping Artifactory with the default admin:password credentials
	artDetails := config.ServerDetails{ArtifactoryUrl: clientUtils.AddTrailingSlashIfNeeded(tcb.SourceServerDetails.ArtifactoryUrl), User: defaultAdminUsername, Password: defaultAdminPassword}
	servicesManager, err := utils.CreateServiceManager(&artDetails, -1, 0, false)
	if err != nil {
		return false, err
	}

	// This cannot be executed with commands.Exec()! Doing so will cause usage report being sent with admin:password as well.
	if _, err = servicesManager.Ping(); err == nil {
		log.Output()
		log.Warn("The default 'admin:password' credentials are used by a configured user in your source platform.\n" +
			"Those credentials will be transferred to your SaaS target platform.")
		return true, nil
	}

	// If the password of the admin user is not the default one, reset the failed login attempts counter in Artifactory
	// by unlocking the user. We do that to avoid login suspension to the admin user.
	return false, tcb.SourceArtifactoryManager.UnlockUser(defaultAdminUsername)
}

func (tcb *TransferConfigBase) LogTitle(title string) {
	log.Info(coreutils.PrintBoldTitle(fmt.Sprintf("========== %s ==========", title)))
}

// Remove federated members from the input federated repository.
// federatedRepoParams - Federated repository parameters
func (tcb *TransferConfigBase) removeFederatedMembers(federatedRepoParams interface{}) (interface{}, error) {
	repoMap, err := InterfaceToMap(federatedRepoParams)
	if err != nil {
		return nil, err
	}
	if _, exist := repoMap["members"]; exist {
		delete(repoMap, "members")
		tcb.FederatedMembersRemoved = true
	} else {
		return federatedRepoParams, nil
	}
	return MapToInterface(repoMap)
}

// Create a repository in the target server and assign the repository to the required project, if any.
// repoParams - Repository parameters
// repoKey    - Repository key
func (tcb *TransferConfigBase) createRepositoryAndAssignToProject(repoParams interface{}, repoDetails services.RepositoryDetails) (err error) {
	var projectKey string
	if repoParams, projectKey, err = removeProjectKeyIfNeeded(repoParams, repoDetails.Key); err != nil {
		return
	}
	if projectKey != "" {
		// Workaround - It's possible that the repository could be assigned to a project in the access.bootstrap.json.
		// This is why we make sure to detach it before actually creating the repository.
		// If the project isn't linked to the repository, an error might come up, but we ignore it because we can't
		// be certain whether the repository was actually assigned to the project or not.
		_ = tcb.TargetAccessManager.UnassignRepoFromProject(repoDetails.Key)
	}
	if err = tcb.TargetArtifactoryManager.CreateRepositoryWithParams(repoParams, repoDetails.Key); err != nil {
		return
	}
	if projectKey != "" {
		return tcb.TargetAccessManager.AssignRepoToProject(repoDetails.Key, projectKey, true)
	}
	return
}

// Remove non-default project key from the repository parameters if existed and the repository key does not start with it.
// This is needed to allow creating repository assigned to projects so that the repository name is not starting with the project key prefix.
// Returns the updated repository params, the project key if removed and an error if any.
func removeProjectKeyIfNeeded(repoParams interface{}, repoKey string) (interface{}, string, error) {
	var projectKey string
	repoMap, err := InterfaceToMap(repoParams)
	if err != nil {
		return nil, "", err
	}
	if value, exist := repoMap["projectKey"]; exist {
		var ok bool
		if projectKey, ok = value.(string); !ok {
			return nil, "", errorutils.CheckErrorf("couldn't parse the 'projectKey' value '%v' of repository '%s'", value, repoKey)
		}
		if projectKey == "default" || strings.HasPrefix(repoKey, projectKey+"-") {
			// The repository key is starting with the project key prefix:
			// <project-key>-<repo-name>
			return repoParams, "", nil
		}
		delete(repoMap, "projectKey")
	} else {
		return repoParams, "", nil
	}
	response, err := MapToInterface(repoMap)
	if err != nil {
		return nil, "", err
	}
	return response, projectKey, errorutils.CheckError(err)
}

// During the transfer-config commands we remove federated members, if existed.
// This method log an info that the federated members should be reconfigured in the target server.
func (tcb *TransferConfigBase) LogIfFederatedMemberRemoved() {
	if tcb.FederatedMembersRemoved {
		log.Info("☝️  Your Federated repositories have been transferred to your target instance, but their members have been removed on the target.\n",
			"You should add members to your Federated repositories on your target instance as described here:",
			coreutils.JFrogHelpUrl+"jfrog-artifactory-documentation/federated-repositories")
	}
}

// Convert the input JSON interface to a map.
// jsonInterface - JSON interface, such as repository params
func InterfaceToMap(jsonInterface interface{}) (map[string]interface{}, error) {
	b, err := json.Marshal(jsonInterface)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	newMap := make(map[string]interface{})
	err = errorutils.CheckError(json.Unmarshal(b, &newMap))
	return newMap, err
}

// Convert the input map to JSON interface.
// mapToTransfer - Map of string to interface, such as repository name
func MapToInterface(mapToTransfer map[string]interface{}) (interface{}, error) {
	repoBytes, err := json.Marshal(mapToTransfer)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	var response interface{}
	err = json.Unmarshal(repoBytes, &response)
	return response, errorutils.CheckError(err)
}
