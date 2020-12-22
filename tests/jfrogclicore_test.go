package tests

import (
	"os"
	"testing"

	"github.com/jfrog/jfrog-cli-core/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/utils/tests"
	"github.com/jfrog/jfrog-client-go/utils/log"
	clientTests "github.com/jfrog/jfrog-client-go/utils/tests"
)

const (
	JfrogTestsHome       = ".jfrogCliCoreTest"
	CoreIntegrationTests = "github.com/jfrog/jfrog-cli-core/tests"
)

func TestUnitTests(t *testing.T) {
	oldHome, err := tests.SetJfrogHome()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	defer os.Setenv(coreutils.HomeDir, oldHome)
	defer tests.CleanUnitTestsJfrogHome()

	packages := clientTests.GetTestPackages("./../...")
	packages = clientTests.ExcludeTestsPackage(packages, CoreIntegrationTests)
	clientTests.RunTests(packages, false)
}
