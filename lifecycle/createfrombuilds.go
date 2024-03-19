package lifecycle

import (
	"encoding/json"
	rtUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	rtServices "github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/lifecycle"
	"github.com/jfrog/jfrog-client-go/lifecycle/services"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
)

func (rbc *ReleaseBundleCreateCommand) createFromBuilds(servicesManager *lifecycle.LifecycleServicesManager,
	rbDetails services.ReleaseBundleDetails, queryParams services.CommonOptionalQueryParams) error {

	var buildsSource services.CreateFromBuildsSource
	var err error
	if rbc.buildsSpecPath != "" {
		buildsSource, err = rbc.getBuildSourceFromBuildsSpec()
	} else {
		buildsSource, err = rbc.convertSpecToBuildsSource(rbc.spec.Files)
	}
	if err != nil {
		return err
	}

	if len(buildsSource.Builds) == 0 {
		return errorutils.CheckErrorf("at least one build is expected in order to create a release bundle from builds")
	}

	return servicesManager.CreateReleaseBundleFromBuilds(rbDetails, queryParams, rbc.signingKeyName, buildsSource)
}

func (rbc *ReleaseBundleCreateCommand) getBuildSourceFromBuildsSpec() (buildsSource services.CreateFromBuildsSource, err error) {
	builds := CreateFromBuildsSpec{}
	content, err := fileutils.ReadFile(rbc.buildsSpecPath)
	if err != nil {
		return
	}
	if err = json.Unmarshal(content, &builds); errorutils.CheckError(err) != nil {
		return
	}

	return rbc.convertBuildsSpecToBuildsSource(builds)
}

func (rbc *ReleaseBundleCreateCommand) convertBuildsSpecToBuildsSource(builds CreateFromBuildsSpec) (services.CreateFromBuildsSource, error) {
	buildsSource := services.CreateFromBuildsSource{}
	for _, build := range builds.Builds {
		buildSource := services.BuildSource{BuildName: build.Name, IncludeDependencies: build.IncludeDependencies}
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

func (rbc *ReleaseBundleCreateCommand) convertSpecToBuildsSource(files []spec.File) (services.CreateFromBuildsSource, error) {
	buildsSource := services.CreateFromBuildsSource{}
	for _, file := range files {
		buildName, buildNumber, err := rbc.getBuildDetailsFromIdentifier(file.Build, file.Project)
		if err != nil {
			return services.CreateFromBuildsSource{}, err
		}
		isIncludeDeps, err := file.IsIncludeDeps(false)
		if err != nil {
			return services.CreateFromBuildsSource{}, err
		}

		buildSource := services.BuildSource{
			BuildName:           buildName,
			BuildNumber:         buildNumber,
			BuildRepository:     utils.GetBuildInfoRepositoryByProject(file.Project),
			IncludeDependencies: isIncludeDeps,
		}
		buildsSource.Builds = append(buildsSource.Builds, buildSource)
	}
	return buildsSource, nil
}

func (rbc *ReleaseBundleCreateCommand) getLatestBuildNumberIfEmpty(buildName, buildNumber, project string) (string, error) {
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

func (rbc *ReleaseBundleCreateCommand) getBuildDetailsFromIdentifier(buildIdentifier, project string) (string, string, error) {
	aqlService, err := rbc.getAqlService()
	if err != nil {
		return "", "", err
	}

	buildName, buildNumber, err := utils.GetBuildNameAndNumberFromBuildIdentifier(buildIdentifier, project, aqlService)
	if err != nil {
		return "", "", err
	}
	if buildName == "" || buildNumber == "" {
		return "", "", errorutils.CheckErrorf("could not identify a build info by the '%s' identifier in artifactory", buildIdentifier)
	}
	return buildName, buildNumber, nil
}

func (rbc *ReleaseBundleCreateCommand) getAqlService() (*rtServices.AqlService, error) {
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
	Name                string `json:"name,omitempty"`
	Number              string `json:"number,omitempty"`
	Project             string `json:"project,omitempty"`
	IncludeDependencies bool   `json:"includeDependencies,omitempty"`
}
