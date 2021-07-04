package tests

import (
	"os"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
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
	oldHome, err := tests.SetJfrogHome()
	if err != nil {
		clientLog.Error(err)
		os.Exit(1)
	}
	defer os.Setenv(coreutils.HomeDir, oldHome)
	defer tests.CleanUnitTestsJfrogHome()

	packages := clientTests.GetTestPackages("./../...")
	packages = clientTests.ExcludeTestsPackage(packages, CoreIntegrationTests)
	clientTests.RunTests(packages, false)
}
