package lifecycle

import (
	"encoding/json"
	"github.com/jfrog/jfrog-client-go/lifecycle"
	"github.com/jfrog/jfrog-client-go/lifecycle/services"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
)

func (rbc *ReleaseBundleCreate) createFromReleaseBundles(servicesManager *lifecycle.LifecycleServicesManager,
	rbDetails services.ReleaseBundleDetails, params services.CreateOrPromoteReleaseBundleParams) error {

	bundles := CreateFromReleaseBundlesSpec{}
	content, err := fileutils.ReadFile(rbc.releaseBundlesSpecPath)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(content, &bundles); err != nil {
		return errorutils.CheckError(err)
	}

	if len(bundles.ReleaseBundles) == 0 {
		return errorutils.CheckErrorf("at least one release bundle is expected in order to create a release bundle from release bundles")
	}

	releaseBundlesSource := rbc.convertToReleaseBundlesSource(bundles)
	return servicesManager.CreateReleaseBundleFromBundles(rbDetails, params, releaseBundlesSource)
}

func (rbc *ReleaseBundleCreate) convertToReleaseBundlesSource(bundles CreateFromReleaseBundlesSpec) services.CreateFromReleaseBundlesSource {
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

type CreateFromReleaseBundlesSpec struct {
	ReleaseBundles []SourceReleaseBundleSpec `json:"releaseBundles,omitempty"`
}

type SourceReleaseBundleSpec struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	Project string `json:"project,omitempty"`
}
