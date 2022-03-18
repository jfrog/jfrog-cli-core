package tests

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/log"
	clientLog "github.com/jfrog/jfrog-client-go/utils/log"

	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"

	clientTests "github.com/jfrog/jfrog-client-go/utils/tests"
)

const (
	CoreIntegrationTests = "github.com/jfrog/jfrog-cli-core/v2/tests"
)

func init() {
	log.SetDefaultLogger()
}

func TestUnitTests(t *testing.T) {
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	if err != nil {
		clientLog.Error(err)
		os.Exit(1)
	}
	defer cleanUpJfrogHome()

	packages := clientTests.GetTestPackages("./../...")
	packages = clientTests.ExcludeTestsPackage(packages, CoreIntegrationTests)
	assert.NoError(t, clientTests.RunTests(packages, false))
}
