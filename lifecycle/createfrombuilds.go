package lifecycle

import (
	"encoding/json"
	rtUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	rtServices "github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/lifecycle"
	"github.com/jfrog/jfrog-client-go/lifecycle/services"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
)

func (rbc *ReleaseBundleCreate) createFromBuilds(servicesManager *lifecycle.LifecycleServicesManager,
	rbDetails services.ReleaseBundleDetails, params services.CreateOrPromoteReleaseBundleParams) error {

	builds := CreateFromBuildsSpec{}
	content, err := fileutils.ReadFile(rbc.buildsSpecPath)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(content, &builds); err != nil {
		return errorutils.CheckError(err)
	}

	if len(builds.Builds) == 0 {
		return errorutils.CheckErrorf("at least one build is expected in order to create a release bundle from builds")
	}

	buildsSource, err := rbc.convertToBuildsSource(builds)
	if err != nil {
		return err
	}
	return servicesManager.CreateReleaseBundleFromBuilds(rbDetails, params, buildsSource)
}

func (rbc *ReleaseBundleCreate) convertToBuildsSource(builds CreateFromBuildsSpec) (services.CreateFromBuildsSource, error) {
	buildsSource := services.CreateFromBuildsSource{}
	for _, build := range builds.Builds {
		buildSource := services.BuildSource{BuildName: build.Name}
		buildNumber, err := rbc.getLatestBuildNumberIfEmpty(build.Name, build.Number, build.Project)
		if err != nil {
			return services.CreateFromBuildsSource{}, err
		}
		buildSource.BuildNumber = buildNumber
		buildSource.BuildRepository = utils.GetBuildInfoRepositoryByProject(build.Project)
		buildsSource.Builds = append(buildsSource.Builds, buildSource)
	}
	return buildsSource, nil
}

func (rbc *ReleaseBundleCreate) getLatestBuildNumberIfEmpty(buildName, buildNumber, project string) (string, error) {
	if buildNumber != "" {
		return buildNumber, nil
	}

	aqlService, err := rbc.getAqlService()
	if err != nil {
		return "", err
	}

	buildNumber, err = utils.GetLatestBuildNumberFromArtifactory(buildName, project, aqlService)
	if err != nil {
		return "", err
	}
	if buildNumber == "" {
		return "", errorutils.CheckErrorf("could not find a build info with name '%s' in artifactory", buildName)
	}
	return buildNumber, nil
}

func (rbc *ReleaseBundleCreate) getAqlService() (*rtServices.AqlService, error) {
	rtServiceManager, err := rtUtils.CreateServiceManager(rbc.serverDetails, 3, 0, false)
	if err != nil {
		return nil, err
	}
	return rtServices.NewAqlService(rtServiceManager.GetConfig().GetServiceDetails(), rtServiceManager.Client()), nil
}

type CreateFromBuildsSpec struct {
	Builds []SourceBuildSpec `json:"builds,omitempty"`
}

type SourceBuildSpec struct {
	Name    string `json:"name,omitempty"`
	Number  string `json:"number,omitempty"`
	Project string `json:"project,omitempty"`
}
