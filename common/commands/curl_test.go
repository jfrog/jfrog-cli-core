package commands

import (
	"testing"
)

func TestFindNextArg(t *testing.T) {
	command := &CurlCommand{}
	args := [][]string{
		{"-X", "GET", "arg1", "--foo", "bar"},
		{"-X", "GET", "--server-idea", "foo", "/api/arg2"},
		{"-XGET", "--foo", "bar", "--foo-bar", "meow", "arg3"},
	}

	expected := []struct {
		int
		string
	}{
		{2, "arg1"},
		{4, "/api/arg2"},
		{5, "arg3"},
	}

	for index, test := range args {
		command.arguments = test
		actualArgIndex, actualArg := command.findUriValueAndIndex()

		if actualArgIndex != expected[index].int {
			t.Errorf("Expected arg index of: %d, got: %d.", expected[index].int, actualArgIndex)
		}
		if actualArg != expected[index].string {
			t.Errorf("Expected arg index of: %s, got: %s.", expected[index].string, actualArg)
		}
	}
}

func TestIsCredsFlagExists(t *testing.T) {
	command := &CurlCommand{}
	args := [][]string{
		{"-X", "GET", "arg1", "--foo", "bar", "-uaaa:ppp"},
		{"-X", "GET", "--server-idea", "foo", "-u", "aaa:ppp", "/api/arg2"},
		{"-XGET", "--foo", "bar", "--foo-bar", "--user", "meow", "-Ttest"},
		{"-XGET", "--foo", "bar", "--foo-bar", "-Ttest"},
	}

	expected := []bool{
		true,
		true,
		true,
		false,
	}

	for index, test := range args {
		command.arguments = test
		flagExists := command.isCredentialsFlagExists()

		if flagExists != expected[index] {
			t.Errorf("Expected flag existstence to be: %t, got: %t.", expected[index], flagExists)
		}
	}
}

func TestBuildCommandUrl(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		uriIndex  int
		uriValue  string
		expectErr bool
	}{
		{"test1", []string{"-X", "GET", "/api/build/test1", "--server-id", "test1", "--foo", "bar"}, 2, "https://artifactory:8081/artifactory/api/build/test1", false},
		{"test2", []string{"-X", "GET", "/api/build/test2", "--server-idea", "foo", "--server-id=test2"}, 2, "https://artifactory:8081/artifactory/api/build/test2", false},
		{"test3", []string{"-XGET", "--/api/build/test3", "--server-id="}, 1, "https://artifactory:8081/artifactory/api/build/test3", true},
		{"test4", []string{"-XGET", "-Test4", "--server-id", "bar"}, 1, "https://artifactory:8081/artifactory/api/build/test4", true},
		{"test5", []string{"-X", "GET", "api/build/test5", "--server-id", "test5", "--foo", "bar"}, 2, "https://artifactory:8081/artifactory/api/build/test5", false},
	}

	command := &CurlCommand{}
	urlPrefix := "https://artifactory:8081/artifactory/"
	for _, test := range tests {
		command.arguments = test.arguments
		t.Run(test.name, func(t *testing.T) {
			uriIndex, uriValue, err := command.buildCommandUrl(urlPrefix)

			// Check errors.
			if err != nil && !test.expectErr {
				t.Error(err)
			}
			if err == nil && test.expectErr {
				t.Errorf("Expecting: error, Got: nil")
			}

			if err == nil {
				// Validate results.
				if uriValue != test.uriValue {
					t.Errorf("Expected uri value of: %s, got: %s.", test.uriValue, uriValue)
				}
				if uriIndex != test.uriIndex {
					t.Errorf("Expected uri index of: %d, got: %d.", test.uriIndex, uriIndex)
				}
			}
		})
	}
}

func TestFindUriWithStandaloneFlags(t *testing.T) {
	tests := []struct {
		name             string
		arguments        []string
		expectedUriIndex int
		expectedUri      string
	}{
		{
			name:             "regression_silent_show_error_verbose",
			arguments:        []string{"-s", "--show-error", "api/repositories/dev-master-maven-local", "--verbose"},
			expectedUriIndex: 2,
			expectedUri:      "api/repositories/dev-master-maven-local",
		},
		{
			name:             "bug_case_1_output_location_verbose",
			arguments:        []string{"-o", "helm.tar.gz", "-L", "-vvv", "helm-sh/helm-v3.19.0-linux-amd64.tar.gz"},
			expectedUriIndex: 4,
			expectedUri:      "helm-sh/helm-v3.19.0-linux-amd64.tar.gz",
		},
		{
			name:             "bug_case_2_output_verbose_location",
			arguments:        []string{"-o", "helm.tar.gz", "-vvv", "-L", "helm-sh/helm-v3.19.0-linux-amd64.tar.gz"},
			expectedUriIndex: 4,
			expectedUri:      "helm-sh/helm-v3.19.0-linux-amd64.tar.gz",
		},
		{
			name:             "bug_case_3_output_location_silent",
			arguments:        []string{"-o", "helm.tar.gz", "-L", "-s", "helm-sh/helm-v3.19.0-linux-amd64.tar.gz"},
			expectedUriIndex: 4,
			expectedUri:      "helm-sh/helm-v3.19.0-linux-amd64.tar.gz",
		},
		{
			name:             "bug_case_4_location_output",
			arguments:        []string{"-L", "-o", "helm.tar.gz", "helm-sh/helm-v3.19.0-linux-amd64.tar.gz"},
			expectedUriIndex: 3,
			expectedUri:      "helm-sh/helm-v3.19.0-linux-amd64.tar.gz",
		},
		{
			name:             "bug_case_5_output_location",
			arguments:        []string{"-o", "helm.tar.gz", "-L", "helm-sh/helm-v3.19.0-linux-amd64.tar.gz"},
			expectedUriIndex: 3,
			expectedUri:      "helm-sh/helm-v3.19.0-linux-amd64.tar.gz",
		},
		{
			name:             "bug_case_6_location_verbose_output",
			arguments:        []string{"-L", "-vvv", "-o", "helm.tar.gz", "helm-sh/helm-v3.19.0-linux-amd64.tar.gz"},
			expectedUriIndex: 4,
			expectedUri:      "helm-sh/helm-v3.19.0-linux-amd64.tar.gz",
		},
		{
			name:             "bug_case_7_location_output_verbose",
			arguments:        []string{"-L", "-o", "helm.tar.gz", "-vvv", "helm-sh/helm-v3.19.0-linux-amd64.tar.gz"},
			expectedUriIndex: 4,
			expectedUri:      "helm-sh/helm-v3.19.0-linux-amd64.tar.gz",
		},
		{
			name:             "multiple_standalone_flags_combined",
			arguments:        []string{"-sS", "-L", "api/system/ping"},
			expectedUriIndex: 2,
			expectedUri:      "api/system/ping",
		},
		{
			name:             "long_standalone_flags",
			arguments:        []string{"--silent", "--show-error", "--location", "api/system/ping"},
			expectedUriIndex: 3,
			expectedUri:      "api/system/ping",
		},
		{
			name:             "mixed_short_long_standalone",
			arguments:        []string{"-X", "GET", "-H", "Content-Type: application/json", "--verbose", "--insecure", "api/repositories"},
			expectedUriIndex: 6,
			expectedUri:      "api/repositories",
		},
		{
			name:             "inline_short_flag_value",
			arguments:        []string{"-XPOST", "-HContent-Type:application/json", "-L", "api/repositories"},
			expectedUriIndex: 3,
			expectedUri:      "api/repositories",
		},
		{
			name:             "long_flag_with_equals",
			arguments:        []string{"--request=GET", "--header=Accept:application/json", "-v", "api/system/ping"},
			expectedUriIndex: 3,
			expectedUri:      "api/system/ping",
		},
	}

	command := &CurlCommand{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command.arguments = test.arguments
			actualIndex, actualUri := command.findUriValueAndIndex()

			if actualIndex != test.expectedUriIndex {
				t.Errorf("Expected URI index: %d, got: %d. Arguments: %v", test.expectedUriIndex, actualIndex, test.arguments)
			}
			if actualUri != test.expectedUri {
				t.Errorf("Expected URI: %s, got: %s. Arguments: %v", test.expectedUri, actualUri, test.arguments)
			}
		})
	}
}
