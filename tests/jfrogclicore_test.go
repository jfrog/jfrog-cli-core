package tests

import (
	"github.com/jfrog/jfrog-cli-core/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/utils/tests"
	"github.com/jfrog/jfrog-client-go/utils/log"
	clientTests "github.com/jfrog/jfrog-client-go/utils/tests"
	"os"
	"path/filepath"
	"testing"
)

const (
	JfrogTestsHome       = ".jfrogCliCoreTest"
	CoreIntegrationTests = "github.com/jfrog/jfrog-cli-core/tests"
)

func TestUnitTests(t *testing.T) {
	homePath, err := filepath.Abs(JfrogTestsHome)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	oldHome, err := tests.SetJfrogHome(homePath)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	defer os.Setenv(coreutils.HomeDir, oldHome)

	packages := clientTests.GetTestPackages("./../...")
	packages = clientTests.ExcludeTestsPackage(packages, CoreIntegrationTests)
	clientTests.RunTests(packages, false)
	tests.CleanUnitTestsJfrogHome(homePath)
}
