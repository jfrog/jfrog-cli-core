package tests

import (
	"os"
	"testing"

	"github.com/jfrog/jfrog-cli-core/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/utils/log"
	clientLog "github.com/jfrog/jfrog-client-go/utils/log"

	"github.com/jfrog/jfrog-cli-core/utils/tests"

	clientTests "github.com/jfrog/jfrog-client-go/utils/tests"
)

const (
	CoreIntegrationTests = "github.com/jfrog/jfrog-cli-core/tests"
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
