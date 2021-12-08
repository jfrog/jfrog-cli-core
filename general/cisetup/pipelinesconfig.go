package cisetup

import (
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/auth"
	clientConfig "github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/pipelines"
	"github.com/jfrog/jfrog-client-go/pipelines/services"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	PipelinesYamlName = "pipelines.yml"
)

type JFrogPipelinesConfigurator struct {
	SetupData      *CiSetupData
	PipelinesToken string
}

func (pc *JFrogPipelinesConfigurator) Config() (vcsIntName, rtIntName string, err error) {
	log.Info("Configuring JFrog Pipelines...")
	serviceDetails, err := config.GetSpecificConfig(ConfigServerId, false, false)
	if err != nil {
		return "", "", err
	}

	psm, err := pc.createPipelinesServiceManager(serviceDetails)
	if err != nil {
		return "", "", err
	}

	vcsIntName, vcsIntId, err := pc.createVcsIntegration(psm)
	if err != nil {
		return "", "", err
	}

	rtIntName, err = pc.createArtifactoryIntegration(psm, serviceDetails)
	if err != nil {
		return "", "", err
	}

	err = pc.doAddPipelineSource(psm, vcsIntId)
	if err != nil {
		return "", "", err
	}
	return vcsIntName, rtIntName, nil
}

func (pc *JFrogPipelinesConfigurator) doAddPipelineSource(psm *pipelines.PipelinesServicesManager, projectIntegrationId int) (err error) {
	_, err = psm.AddPipelineSource(projectIntegrationId, pc.SetupData.GetRepoFullName(), pc.SetupData.GitBranch, PipelinesYamlName)
	if err != nil {
		// If source already exists, ignore error.
		if _, ok := err.(*services.SourceAlreadyExistsError); ok {
			log.Debug("Pipeline Source for repo '" + pc.SetupData.GetRepoFullName() + "' and branch '" + pc.SetupData.GitBranch + "' already exists and will be used.")
			err = nil
		}
	}
	return
}

func (pc *JFrogPipelinesConfigurator) createPipelinesServiceManager(details *config.ServerDetails) (*pipelines.PipelinesServicesManager, error) {
	// Create new details with pipelines token.
	pipelinesDetails := *details
	pipelinesDetails.AccessToken = pc.PipelinesToken
	pipelinesDetails.User = ""
	pipelinesDetails.Password = ""

	certsPath, err := coreutils.GetJfrogCertsDir()
	if err != nil {
		return nil, err
	}
	pAuth, err := pipelinesDetails.CreatePipelinesAuthConfig()
	if err != nil {
		return nil, err
	}
	serviceConfig, err := clientConfig.NewConfigBuilder().
		SetServiceDetails(pAuth).
		SetCertificatesPath(certsPath).
		SetInsecureTls(pipelinesDetails.InsecureTls).
		SetDryRun(false).
		Build()
	if err != nil {
		return nil, err
	}
	return pipelines.New(serviceConfig)
}

func (pc *JFrogPipelinesConfigurator) createVcsIntegration(psm *pipelines.PipelinesServicesManager) (integrationName string, integrationId int, err error) {
	switch pc.SetupData.GitProvider {
	case Github:
		integrationName = pc.createIntegrationName(services.GithubName)
		integrationId, err = psm.CreateGithubIntegration(integrationName, pc.SetupData.VcsCredentials.AccessToken)
	case GithubEnterprise:
		integrationName = pc.createIntegrationName(services.GithubEnterpriseName)
		integrationId, err = psm.CreateGithubEnterpriseIntegration(integrationName, pc.SetupData.VcsBaseUrl, pc.SetupData.VcsCredentials.AccessToken)
	case Bitbucket:
		integrationName = pc.createIntegrationName(services.BitbucketName)
		integrationId, err = psm.CreateBitbucketIntegration(integrationName, pc.SetupData.VcsCredentials.User, pc.SetupData.VcsCredentials.AccessToken)
	case BitbucketServer:
		integrationName = pc.createIntegrationName(services.BitbucketServerName)
		cred := pc.SetupData.VcsCredentials.AccessToken
		if cred == "" {
			cred = pc.SetupData.VcsCredentials.Password
		}
		integrationId, err = psm.CreateBitbucketServerIntegration(integrationName, pc.SetupData.VcsBaseUrl, pc.SetupData.VcsCredentials.User, cred)
	case Gitlab:
		integrationName = pc.createIntegrationName(services.GitlabName)
		integrationId, err = psm.CreateGitlabIntegration(integrationName, pc.SetupData.VcsBaseUrl, pc.SetupData.VcsCredentials.AccessToken)
	default:
		return "", -1, errorutils.CheckErrorf("vcs type is not supported at the moment")
	}
	// If no error, or unexpected error, return.
	if err == nil {
		return
	}
	if _, ok := err.(*services.IntegrationAlreadyExistsError); !ok {
		return
	}

	// If integration already exists, get the id from pipelines.
	log.Debug("Integration '" + integrationName + "' already exists and will be used. Fetching id from pipelines...")
	integration, err := psm.GetIntegrationByName(integrationName)
	if err != nil {
		return
	}
	integrationId = integration.Id
	return
}

func (pc *JFrogPipelinesConfigurator) createArtifactoryIntegration(psm *pipelines.PipelinesServicesManager, details *config.ServerDetails) (string, error) {
	integrationName := pc.createIntegrationName("rt")
	apiKey, err := pc.getApiKey(details)
	if err != nil {
		return "", err
	}
	user := details.User
	if user == "" {
		user, err = auth.ExtractUsernameFromAccessToken(details.AccessToken)
		if err != nil {
			return "", err
		}
	}
	_, err = psm.CreateArtifactoryIntegration(integrationName, details.ArtifactoryUrl, user, apiKey)
	if err != nil {
		// If integration already exists, ignore error.
		if _, ok := err.(*services.IntegrationAlreadyExistsError); ok {
			log.Debug("Integration '" + integrationName + "' already exists and will be used.")
			err = nil
		}
	}
	return integrationName, err
}

// Get API Key if exists, generate one if needed.
func (pc *JFrogPipelinesConfigurator) getApiKey(details *config.ServerDetails) (string, error) {
	// Try getting API Key for the user if exists.
	asm, err := pc.createRtServiceManager(details)
	if err != nil {
		return "", err
	}
	apiKey, err := asm.GetAPIKey()
	if err != nil || apiKey != "" {
		return apiKey, err
	}

	// Generate API Key for the user.
	return asm.CreateAPIKey()
}

func (pc *JFrogPipelinesConfigurator) createIntegrationName(intType string) string {
	return intType + "_" + createPipelinesSuitableName(pc.SetupData, "integration")
}

func (pc *JFrogPipelinesConfigurator) createRtServiceManager(artDetails *config.ServerDetails) (artifactory.ArtifactoryServicesManager, error) {
	certsPath, err := coreutils.GetJfrogCertsDir()
	if err != nil {
		return nil, err
	}
	artAuth, err := artDetails.CreateArtAuthConfig()
	if err != nil {
		return nil, err
	}
	serviceConfig, err := clientConfig.NewConfigBuilder().
		SetServiceDetails(artAuth).
		SetCertificatesPath(certsPath).
		SetInsecureTls(artDetails.InsecureTls).
		SetDryRun(false).
		Build()
	if err != nil {
		return nil, err
	}
	return artifactory.New(serviceConfig)
}

func createPipelinesSuitableName(data *CiSetupData, suffix string) string {
	name := strings.Join([]string{data.ProjectDomain, data.RepositoryName, suffix}, "_")
	// Pipelines does not allow "-" which might exist in repo names.
	return strings.Replace(name, "-", "_", -1)
}
