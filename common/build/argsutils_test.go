package build

import (
	testsutils "github.com/jfrog/jfrog-client-go/utils/tests"
	"reflect"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
)

func TestExtractBuildDetailsFromArgs(t *testing.T) {
	tests := []struct {
		command             []string
		expectedArgs        []string
		expectedBuildConfig *BuildConfiguration
	}{
		{[]string{"-test", "--build-name", "test1", "--foo", "--build-number", "1", "--module", "module1"}, []string{"-test", "--foo"}, NewBuildConfiguration("test1", "1", "module1", "")},
		{[]string{"--module=module2", "--build-name", "test2", "--foo", "bar", "--build-number=2"}, []string{"--foo", "bar"}, NewBuildConfiguration("test2", "2", "module2", "")},
		{[]string{"foo", "-X", "123", "--build-name", "test3", "--bar", "--build-number=3", "--foox"}, []string{"foo", "-X", "123", "--bar", "--foox"}, NewBuildConfiguration("test3", "3", "", "")},
	}

	for _, test := range tests {
		actualArgs, actualBuildConfig, err := ExtractBuildDetailsFromArgs(test.command)
		if err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(actualArgs, test.expectedArgs) {
			t.Errorf("Expected value: %v, got: %v.", test.expectedArgs, actualArgs)
		}
		if !reflect.DeepEqual(actualBuildConfig, test.expectedBuildConfig) {
			t.Errorf("Expected value: %v, got: %v.", test.expectedBuildConfig, actualBuildConfig)
		}
	}
}

func TestExtractBuildDetailsFromEnv(t *testing.T) {
	const buildNameEnv = "envBuildName"
	const buildNumberEnv = "777"
	tests := []struct {
		command             []string
		expectedArgs        []string
		expectedBuildConfig *BuildConfiguration
	}{
		{[]string{"-test", "--build-name", "test1", "--foo", "--build-number", "1", "--module", "module1"}, []string{"-test", "--foo"}, NewBuildConfiguration("test1", "1", "module1", "")},
		{[]string{"foo", "-X", "123", "--bar", "--build-name=test3", "--build-number=3", "--foox"}, []string{"foo", "-X", "123", "--bar", "--foox"}, NewBuildConfiguration("test3", "3", "", "")},
		{[]string{"foo", "-X", "123", "--bar", "--build-name=test1", "--build-number=1", "--foox"}, []string{"foo", "-X", "123", "--bar", "--foox"}, NewBuildConfiguration("test1", "1", "", "")},
		{[]string{"foo", "-X", "123", "--bar", "--foox"}, []string{"foo", "-X", "123", "--bar", "--foox"}, NewBuildConfiguration(buildNameEnv, buildNumberEnv, "", "")},
	}
	setEnvCallback := testsutils.SetEnvWithCallbackAndAssert(t, coreutils.BuildName, buildNameEnv)
	defer setEnvCallback()
	setEnvCallback = testsutils.SetEnvWithCallbackAndAssert(t, coreutils.BuildNumber, buildNumberEnv)
	defer setEnvCallback()
	for _, test := range tests {
		actualArgs, actualBuildConfig, err := ExtractBuildDetailsFromArgs(test.command)
		if err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(actualArgs, test.expectedArgs) {
			t.Errorf("Expected value: %v, got: %v.", test.expectedArgs, actualArgs)
		}
		if !reflect.DeepEqual(actualBuildConfig, test.expectedBuildConfig) {
			t.Errorf("Expected value: %v, got: %v.", test.expectedBuildConfig, actualBuildConfig)
		}
	}
}
