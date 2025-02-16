package lifecycle

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/lifecycle/services"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetPrerequisites_Success(t *testing.T) {
	serverDetails := &config.ServerDetails{}
	rbCmd := &releaseBundleCmd{
		serverDetails:        serverDetails,
		releaseBundleName:    "testRelease",
		releaseBundleVersion: "1.0.0",
		sync:                 true,
		rbProjectKey:         "project1",
	}

	expectedQueryParams := services.CommonOptionalQueryParams{
		ProjectKey: rbCmd.rbProjectKey,
		Async:      false,
	}

	expectedRbDetails := services.ReleaseBundleDetails{
		ReleaseBundleName:    rbCmd.releaseBundleName,
		ReleaseBundleVersion: rbCmd.releaseBundleVersion,
	}

	servicesManager, rbDetails, queryParams, err := rbCmd.getPrerequisites()

	assert.NoError(t, err)
	assert.NotNil(t, servicesManager, "Expected servicesManager to be initialized")
	assert.Equal(t, expectedRbDetails, rbDetails, "ReleaseBundleDetails does not match expected values")
	assert.Equal(t, expectedQueryParams, queryParams, "QueryParams do not match expected values")

}

func TestGetPromotionPrerequisites_Success(t *testing.T) {
	serverDetails := &config.ServerDetails{}
	rbp := &ReleaseBundlePromoteCommand{
		promotionType: "move",
		releaseBundleCmd: releaseBundleCmd{
			serverDetails:        serverDetails,
			releaseBundleName:    "testRelease",
			releaseBundleVersion: "1.0.0",
			sync:                 true,
			rbProjectKey:         "project1",
		},
	}

	expectedQueryParams := services.CommonOptionalQueryParams{
		ProjectKey:    rbp.rbProjectKey,
		Async:         false,
		PromotionType: rbp.promotionType,
	}

	expectedRbDetails := services.ReleaseBundleDetails{
		ReleaseBundleName:    rbp.releaseBundleName,
		ReleaseBundleVersion: rbp.releaseBundleVersion,
	}

	servicesManager, rbDetails, queryParams, err := rbp.getPromotionPrerequisites()

	assert.NoError(t, err)
	assert.NotNil(t, servicesManager, "Expected servicesManager to be initialized")
	assert.Equal(t, expectedRbDetails, rbDetails, "ReleaseBundleDetails do not match expected values") // Replace _ with appropriate variable.
	assert.Equal(t, expectedQueryParams, queryParams, "QueryParams do not match expected values")
}
