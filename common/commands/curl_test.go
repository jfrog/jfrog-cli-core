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
		{"basicGetWithLeadingSlash", []string{"-X", "GET", "/api/build/test1", "--server-id", "test1", "--foo", "bar"}, 2, "https://artifactory:8081/artifactory/api/build/test1", false},
		{"getWithInlineServerId", []string{"-X", "GET", "/api/build/test2", "--server-idea", "foo", "--server-id=test2"}, 2, "https://artifactory:8081/artifactory/api/build/test2", false},
		{"uriStartsWithDashDash", []string{"-XGET", "--/api/build/test3", "--server-id="}, 1, "https://artifactory:8081/artifactory/api/build/test3", true},
		{"uriStartsWithDash", []string{"-XGET", "-Test4", "--server-id", "bar"}, 1, "https://artifactory:8081/artifactory/api/build/test4", true},
		{"basicGetWithoutLeadingSlash", []string{"-X", "GET", "api/build/test5", "--server-id", "test5", "--foo", "bar"}, 2, "https://artifactory:8081/artifactory/api/build/test5", false},
		{"fullHTTP", []string{"-L", "http://example.com/api/test"}, -1, "", true},
		{"fullHTTPS", []string{"-L", "https://example.com/api/test"}, -1, "", true},
		{"noURI", []string{"-X", "GET", "-H", "Auth: token"}, -1, "", true},
		{"onlyFlags", []string{"-L", "-v", "-s", "--insecure"}, -1, "", true},
		{"specialChars", []string{"-X", "GET", "/api/test?param=value&foo=bar"}, 2, "https://artifactory:8081/artifactory/api/test?param=value&foo=bar", false},
		{"uriWithSpaces", []string{"-X", "GET", "/api/test%20space"}, 2, "https://artifactory:8081/artifactory/api/test%20space", false},
		{"multipleURIs", []string{"api/first", "-X", "GET", "api/second"}, 0, "https://artifactory:8081/artifactory/api/first", false},
		{"dashInURI", []string{"-X", "GET", "-not-a-flag/api/test"}, -1, "", true},
		{"booleanFlags", []string{"-L", "-k", "-v", "-s", "api/test"}, 4, "https://artifactory:8081/artifactory/api/test", false},
		{"longBooleanFlags", []string{"--location", "--insecure", "--verbose", "api/test"}, 3, "https://artifactory:8081/artifactory/api/test", false},
		{"embeddedValue", []string{"--header=Authorization: Bearer token", "-XPOST", "api/test"}, 2, "https://artifactory:8081/artifactory/api/test", false},
		{"multipleEmbedded", []string{"--data=json", "--header=Content-Type: application/json", "api/test"}, 2, "https://artifactory:8081/artifactory/api/test", false},
		{"combinedBoolean", []string{"-Lkvs", "api/test"}, 1, "https://artifactory:8081/artifactory/api/test", false},
		{"combinedMixed", []string{"-LvX", "POST", "api/test"}, 2, "https://artifactory:8081/artifactory/api/test", false},
		{"trailingValueFlag", []string{"api/test", "-X"}, 0, "https://artifactory:8081/artifactory/api/test", false},
		{"trailingBooleanFlag", []string{"api/test", "-L"}, 0, "https://artifactory:8081/artifactory/api/test", false},
		{"emptyArgs", []string{"-X", "GET", "", "api/test"}, 2, "https://artifactory:8081/artifactory/", false},
		{"emptyFlagValue", []string{"-H", "", "api/test"}, 2, "https://artifactory:8081/artifactory/api/test", false},
		{"uriFirst", []string{"api/test", "-X", "GET", "-L"}, 0, "https://artifactory:8081/artifactory/api/test", false},
		{"realWorld", []string{"-sS", "-L", "-X", "POST", "-H", "Content-Type: application/json", "-d", `{"key":"value"}`, "--insecure", "api/repos/test"}, 9, "https://artifactory:8081/artifactory/api/repos/test", false},
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

func TestFindUriWithBooleanFlags(t *testing.T) {
	tests := []struct {
		name             string
		arguments        []string
		expectedUriIndex int
		expectedUri      string
	}{
		{
			name:             "shortSilentWithLongVerbose",
			arguments:        []string{"-s", "--show-error", "api/repositories/dev-master-maven-local", "--verbose"},
			expectedUriIndex: 2,
			expectedUri:      "api/repositories/dev-master-maven-local",
		},
		{
			name:             "outputFlagWithCombinedVerbose",
			arguments:        []string{"-o", "helm.tar.gz", "-L", "-vvv", "helm-sh/helm-v3.19.0-linux-amd64.tar.gz"},
			expectedUriIndex: 4,
			expectedUri:      "helm-sh/helm-v3.19.0-linux-amd64.tar.gz",
		},
		{
			name:             "combinedVerboseBeforeLocation",
			arguments:        []string{"-o", "helm.tar.gz", "-vvv", "-L", "helm-sh/helm-v3.19.0-linux-amd64.tar.gz"},
			expectedUriIndex: 4,
			expectedUri:      "helm-sh/helm-v3.19.0-linux-amd64.tar.gz",
		},
		{
			name:             "outputWithLocationAndSilent",
			arguments:        []string{"-o", "helm.tar.gz", "-L", "-s", "helm-sh/helm-v3.19.0-linux-amd64.tar.gz"},
			expectedUriIndex: 4,
			expectedUri:      "helm-sh/helm-v3.19.0-linux-amd64.tar.gz",
		},
		{
			name:             "locationFirstThenOutput",
			arguments:        []string{"-L", "-o", "helm.tar.gz", "helm-sh/helm-v3.19.0-linux-amd64.tar.gz"},
			expectedUriIndex: 3,
			expectedUri:      "helm-sh/helm-v3.19.0-linux-amd64.tar.gz",
		},
		{
			name:             "outputThenLocationSimple",
			arguments:        []string{"-o", "helm.tar.gz", "-L", "helm-sh/helm-v3.19.0-linux-amd64.tar.gz"},
			expectedUriIndex: 3,
			expectedUri:      "helm-sh/helm-v3.19.0-linux-amd64.tar.gz",
		},
		{
			name:             "locationCombinedVerboseThenOutput",
			arguments:        []string{"-L", "-vvv", "-o", "helm.tar.gz", "helm-sh/helm-v3.19.0-linux-amd64.tar.gz"},
			expectedUriIndex: 4,
			expectedUri:      "helm-sh/helm-v3.19.0-linux-amd64.tar.gz",
		},
		{
			name:             "locationOutputThenCombinedVerbose",
			arguments:        []string{"-L", "-o", "helm.tar.gz", "-vvv", "helm-sh/helm-v3.19.0-linux-amd64.tar.gz"},
			expectedUriIndex: 4,
			expectedUri:      "helm-sh/helm-v3.19.0-linux-amd64.tar.gz",
		},
		{
			name:             "combinedSilentShowErrorWithLocation",
			arguments:        []string{"-sS", "-L", "api/system/ping"},
			expectedUriIndex: 2,
			expectedUri:      "api/system/ping",
		},
		{
			name:             "longFormSilentShowErrorLocation",
			arguments:        []string{"--silent", "--show-error", "--location", "api/system/ping"},
			expectedUriIndex: 3,
			expectedUri:      "api/system/ping",
		},
		{
			name:             "getWithHeaderAndBooleanFlags",
			arguments:        []string{"-X", "GET", "-H", "Content-Type: application/json", "--verbose", "--insecure", "api/repositories"},
			expectedUriIndex: 6,
			expectedUri:      "api/repositories",
		},
		{
			name:             "inlineRequestAndHeaderWithLocation",
			arguments:        []string{"-XPOST", "-HContent-Type:application/json", "-L", "api/repositories"},
			expectedUriIndex: 3,
			expectedUri:      "api/repositories",
		},
		{
			name:             "longFormInlineRequestAndHeader",
			arguments:        []string{"--request=GET", "--header=Accept:application/json", "-v", "api/system/ping"},
			expectedUriIndex: 3,
			expectedUri:      "api/system/ping",
		},
		{
			name:             "allShortBooleanFlags",
			arguments:        []string{"-#", "-0", "-1", "-2", "-3", "-4", "-6", "-a", "-B", "-f", "-g", "-G", "-I", "-i", "api/test"},
			expectedUriIndex: 14,
			expectedUri:      "api/test",
		},
		{
			name:             "mixedKnownUnknownFlags",
			arguments:        []string{"-L", "-9", "possibleValue", "api/test"},
			expectedUriIndex: 3,
			expectedUri:      "api/test",
		},
		{
			name:             "complexCombinedFlags",
			arguments:        []string{"-sLkvo", "output.txt", "api/test"},
			expectedUriIndex: 2,
			expectedUri:      "api/test",
		},
		{
			name:             "httpMethodsAsValues",
			arguments:        []string{"-X", "PATCH", "-X", "OPTIONS", "-X", "TRACE", "api/test"},
			expectedUriIndex: 6,
			expectedUri:      "api/test",
		},
		{
			name:             "quotedValues",
			arguments:        []string{"-H", `Authorization: "Bearer token"`, "-H", "Accept: */*", "api/test"},
			expectedUriIndex: 4,
			expectedUri:      "api/test",
		},
		{
			name:             "jsonData",
			arguments:        []string{"-d", `{"test": "value", "array": [1,2,3]}`, "-H", "Content-Type: application/json", "api/test"},
			expectedUriIndex: 4,
			expectedUri:      "api/test",
		},
		{
			name:             "fileReference",
			arguments:        []string{"-d", "@/path/to/file.json", "-T", "/path/to/upload.tar", "api/test"},
			expectedUriIndex: 4,
			expectedUri:      "api/test",
		},
		{
			name:             "repeatedBooleanFlags",
			arguments:        []string{"-v", "-v", "-v", "-L", "-L", "api/test"},
			expectedUriIndex: 5,
			expectedUri:      "api/test",
		},
		{
			name:             "urlEncodedData",
			arguments:        []string{"--data-urlencode", "param=value&other=test", "api/test"},
			expectedUriIndex: 2,
			expectedUri:      "api/test",
		},
		{
			name:             "proxySettings",
			arguments:        []string{"-x", "proxy.server:8080", "-U", "proxyuser:pass", "api/test"},
			expectedUriIndex: 4,
			expectedUri:      "api/test",
		},
		{
			name:             "certAndKeyFlags",
			arguments:        []string{"-E", "/path/to/cert.pem", "--key", "/path/to/key.pem", "api/test"},
			expectedUriIndex: 4,
			expectedUri:      "api/test",
		},
		{
			name:             "rangeHeader",
			arguments:        []string{"-r", "0-1023", "-C", "-", "api/download/file"},
			expectedUriIndex: 4,
			expectedUri:      "api/download/file",
		},
		{
			name:             "userAgentAndReferer",
			arguments:        []string{"-A", "CustomUserAgent/1.0", "-e", "https://referrer.com", "api/test"},
			expectedUriIndex: 4,
			expectedUri:      "api/test",
		},
		{
			name:             "formData",
			arguments:        []string{"-F", "field1=value1", "-F", "field2=@file.txt", "-F", "field3=<file2.txt", "api/upload"},
			expectedUriIndex: 6,
			expectedUri:      "api/upload",
		},
		{
			name:             "cookiesAndJar",
			arguments:        []string{"-b", "cookies.txt", "-c", "newcookies.txt", "-b", "name=value", "api/test"},
			expectedUriIndex: 6,
			expectedUri:      "api/test",
		},
		{
			name:             "speedAndTime",
			arguments:        []string{"-Y", "1000", "-y", "30", "-m", "120", "api/test"},
			expectedUriIndex: 6,
			expectedUri:      "api/test",
		},
		{
			name:             "retryOptions",
			arguments:        []string{"--retry", "3", "--retry-delay", "5", "--retry-max-time", "60", "api/test"},
			expectedUriIndex: 6,
			expectedUri:      "api/test",
		},
		{
			name:             "writeOutFormat",
			arguments:        []string{"-w", `%{http_code}\n`, "-o", "/dev/null", "-s", "api/test"},
			expectedUriIndex: 5,
			expectedUri:      "api/test",
		},
		{
			name:             "ipv6Address",
			arguments:        []string{"-H", "Host: [::1]", "-6", "api/test"},
			expectedUriIndex: 3,
			expectedUri:      "api/test",
		},
		{
			name:             "traceAndDump",
			arguments:        []string{"--trace", "trace.txt", "--trace-ascii", "ascii.txt", "--dump-header", "headers.txt", "api/test"},
			expectedUriIndex: 6,
			expectedUri:      "api/test",
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
