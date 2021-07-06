package utils

import (
	"os"
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
		{[]string{"-test", "--build-name", "test1", "--foo", "--build-number", "1", "--module", "module1"}, []string{"-test", "--foo"}, &BuildConfiguration{BuildName: "test1", BuildNumber: "1", Module: "module1", Project: ""}},
		{[]string{"--module=module2", "--build-name", "test2", "--foo", "bar", "--build-number=2"}, []string{"--foo", "bar"}, &BuildConfiguration{BuildName: "test2", BuildNumber: "2", Module: "module2", Project: ""}},
		{[]string{"foo", "-X", "123", "--build-name", "test3", "--bar", "--build-number=3", "--foox"}, []string{"foo", "-X", "123", "--bar", "--foox"}, &BuildConfiguration{BuildName: "test3", BuildNumber: "3", Module: "", Project: ""}},
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
		{[]string{"-test", "--build-name", "test1", "--foo", "--build-number", "1", "--module", "module1"}, []string{"-test", "--foo"}, &BuildConfiguration{BuildName: "test1", BuildNumber: "1", Module: "module1", Project: ""}},
		{[]string{"foo", "-X", "123", "--bar", "--build-name=test3", "--build-number=3", "--foox"}, []string{"foo", "-X", "123", "--bar", "--foox"}, &BuildConfiguration{BuildName: "test3", BuildNumber: "3", Module: "", Project: ""}},
		{[]string{"foo", "-X", "123", "--bar", "--build-name=test1", "--build-number=1", "--foox"}, []string{"foo", "-X", "123", "--bar", "--foox"}, &BuildConfiguration{BuildName: "test1", BuildNumber: "1", Module: "", Project: ""}},
		{[]string{"foo", "-X", "123", "--bar", "--foox"}, []string{"foo", "-X", "123", "--bar", "--foox"}, &BuildConfiguration{BuildName: buildNameEnv, BuildNumber: buildNumberEnv, Module: "", Project: ""}},
	}

	os.Setenv(coreutils.BuildName, buildNameEnv)
	os.Setenv(coreutils.BuildNumber, buildNumberEnv)
	defer os.Unsetenv(coreutils.BuildName)
	defer os.Unsetenv(coreutils.BuildNumber)
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
