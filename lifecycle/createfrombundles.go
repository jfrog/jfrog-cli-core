package lifecycle

import (
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/lifecycle"
	"github.com/jfrog/jfrog-client-go/lifecycle/services"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
)

func (rbc *ReleaseBundleCreateCommand) createFromReleaseBundles(servicesManager *lifecycle.LifecycleServicesManager,
	rbDetails services.ReleaseBundleDetails, queryParams services.CommonOptionalQueryParams) error {

	var releaseBundlesSource services.CreateFromReleaseBundlesSource
	var err error
	if rbc.releaseBundlesSpecPath != "" {
		releaseBundlesSource, err = rbc.getReleaseBundlesSourceFromBundlesSpec()
	} else {
		releaseBundlesSource, err = rbc.convertSpecToReleaseBundlesSource(rbc.spec.Files)
	}
	if err != nil {
		return err
	}

	if len(releaseBundlesSource.ReleaseBundles) == 0 {
		return errorutils.CheckErrorf("at least one release bundle is expected in order to create a release bundle from release bundles")
	}

	return servicesManager.CreateReleaseBundleFromBundles(rbDetails, queryParams, rbc.signingKeyName, releaseBundlesSource)
}

func (rbc *ReleaseBundleCreateCommand) convertToReleaseBundlesSource(bundles CreateFromReleaseBundlesSpec) services.CreateFromReleaseBundlesSource {
	releaseBundlesSource := services.CreateFromReleaseBundlesSource{}
	for _, rb := range bundles.ReleaseBundles {
		rbSource := services.ReleaseBundleSource{
			ReleaseBundleName:    rb.Name,
			ReleaseBundleVersion: rb.Version,
			ProjectKey:           rb.Project,
		}
		releaseBundlesSource.ReleaseBundles = append(releaseBundlesSource.ReleaseBundles, rbSource)
	}
	return releaseBundlesSource
}

func (rbc *ReleaseBundleCreateCommand) convertSpecToReleaseBundlesSource(files []spec.File) (services.CreateFromReleaseBundlesSource, error) {
	releaseBundlesSource := services.CreateFromReleaseBundlesSource{}
	for _, file := range files {
		name, version, err := utils.ParseNameAndVersion(file.Bundle, false)
		if err != nil {
			return releaseBundlesSource, err
		}
		if name == "" || version == "" {
			return releaseBundlesSource, errorutils.CheckErrorf(
				"invalid release bundle source was provided. Both name and version are mandatory. Provided name: '%s', version: '%s'", name, version)
		}
		rbSource := services.ReleaseBundleSource{
			ReleaseBundleName:    name,
			ReleaseBundleVersion: version,
			ProjectKey:           file.Project,
		}
		releaseBundlesSource.ReleaseBundles = append(releaseBundlesSource.ReleaseBundles, rbSource)
	}
	return releaseBundlesSource, nil
}

func (rbc *ReleaseBundleCreateCommand) getReleaseBundlesSourceFromBundlesSpec() (releaseBundlesSource services.CreateFromReleaseBundlesSource, err error) {
	releaseBundles := CreateFromReleaseBundlesSpec{}
	content, err := fileutils.ReadFile(rbc.releaseBundlesSpecPath)
	if err != nil {
		return
	}
	if err = json.Unmarshal(content, &releaseBundles); errorutils.CheckError(err) != nil {
		return
	}

	return rbc.convertToReleaseBundlesSource(releaseBundles), nil
}

type CreateFromReleaseBundlesSpec struct {
	ReleaseBundles []SourceReleaseBundleSpec `json:"releaseBundles,omitempty"`
}

type SourceReleaseBundleSpec struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	Project string `json:"project,omitempty"`
}
