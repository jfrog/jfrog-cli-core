package lifecycle

import (
	"errors"
	"path"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	rtServicesUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/lifecycle"
	"github.com/jfrog/jfrog-client-go/lifecycle/services"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

func (rbc *ReleaseBundleCreateCommand) createFromArtifacts(lcServicesManager *lifecycle.LifecycleServicesManager,
	rbDetails services.ReleaseBundleDetails, queryParams services.CommonOptionalQueryParams) (err error) {

	rtServicesManager, err := utils.CreateServiceManager(rbc.serverDetails, 3, 0, false)
	if err != nil {
		return err
	}

	log.Info("Searching artifacts...")
	searchResults, callbackFunc, err := utils.SearchFiles(rtServicesManager, rbc.spec)
	defer func() {
		err = errors.Join(err, callbackFunc())
	}()
	if err != nil {
		return err
	}
	artifactsSource, err := aqlResultToArtifactsSource(searchResults)
	if err != nil {
		return err
	}

	return lcServicesManager.CreateReleaseBundleFromArtifacts(rbDetails, queryParams, rbc.signingKeyName, artifactsSource)
}

func aqlResultToArtifactsSource(readers []*content.ContentReader) (artifactsSource services.CreateFromArtifacts, err error) {
	for _, reader := range readers {
		for searchResult := new(rtServicesUtils.ResultItem); reader.NextRecord(searchResult) == nil; searchResult = new(rtServicesUtils.ResultItem) {
			if err != nil {
				return
			}
			artifactsSource.Artifacts = append(artifactsSource.Artifacts, services.ArtifactSource{
				Path:   path.Join(searchResult.Repo, searchResult.Path, searchResult.Name),
				Sha256: searchResult.Sha256,
			})
		}
		if err = reader.GetError(); err != nil {
			return
		}
		reader.Reset()
	}
	return
}
