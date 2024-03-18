package lifecycle

import (
	"fmt"
	"github.com/jfrog/jfrog-client-go/lifecycle"
	"github.com/jfrog/jfrog-client-go/lifecycle/services"
)

func (rbc *ReleaseBundleCreateCommand) createFromAql(servicesManager *lifecycle.LifecycleServicesManager,
	rbDetails services.ReleaseBundleDetails, queryParams services.CommonOptionalQueryParams) error {
	aqlQuery := fmt.Sprintf(`items.find(%s)`, rbc.spec.Get(0).Aql.ItemsFind)
	return servicesManager.CreateReleaseBundleFromAql(rbDetails, queryParams, rbc.signingKeyName, aqlQuery)
}
