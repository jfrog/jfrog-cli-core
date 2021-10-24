package yarn

import (
	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/gofrog/parallel"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/stretchr/testify/assert"
	"os"
	"reflect"
	"testing"
)

func TestValidateSupportedCommand(t *testing.T) {
	yarnCmd := NewYarnCommand()

	testCases := []struct {
		args  []string
		valid bool
	}{
		{[]string{}, true},
		{[]string{"--json"}, true},
		{[]string{"npm", "publish", "--json"}, false},
		{[]string{"npm", "--json", "publish"}, false},
		{[]string{"npm", "tag", "list"}, false},
		{[]string{"npm", "info", "package-name"}, true},
		{[]string{"npm", "whoami"}, true},
	}

	for _, testCase := range testCases {
		yarnCmd.yarnArgs = testCase.args
		err := yarnCmd.validateSupportedCommand()
		assert.Equal(t, testCase.valid, err == nil, "Test args:", testCase.args)
	}
}

func TestSetAndRestoreEnvironmentVariables(t *testing.T) {
	const jfrogCliTestingEnvVar = "JFROG_CLI_ENV_VAR_FOR_TESTING"
	yarnCmd := &YarnCommand{envVarsBackup: make(map[string]*string)}

	// Check backup and restore of an existing variable
	err := os.Setenv(jfrogCliTestingEnvVar, "abc")
	assert.NoError(t, err)
	err = yarnCmd.backupAndSetEnvironmentVariable(jfrogCliTestingEnvVar, "new-value")
	assert.NoError(t, err)
	assert.Equal(t, "new-value", os.Getenv(jfrogCliTestingEnvVar))
	err = yarnCmd.restoreEnvironmentVariables()
	assert.NoError(t, err)
	assert.Equal(t, "abc", os.Getenv(jfrogCliTestingEnvVar))

	// Check backup and restore of a variable that doesn't exist
	err = os.Unsetenv(jfrogCliTestingEnvVar)
	assert.NoError(t, err)
	err = yarnCmd.backupAndSetEnvironmentVariable(jfrogCliTestingEnvVar, "another-value")
	assert.NoError(t, err)
	assert.Equal(t, "another-value", os.Getenv(jfrogCliTestingEnvVar))
	err = yarnCmd.restoreEnvironmentVariables()
	assert.NoError(t, err)
	_, exist := os.LookupEnv(jfrogCliTestingEnvVar)
	assert.False(t, exist)
}

func TestYarnDependencyName(t *testing.T) {
	testCases := []struct {
		dependencyValue string
		expectedName    string
	}{
		{"yargs-unparser@npm:2.0.0", "yargs-unparser"},
		{"typescript@patch:typescript@npm%3A3.9.9#builtin<compat/typescript>::version=3.9.9&hash=a45b0e", "typescript"},
		{"@babel/highlight@npm:7.14.0", "@babel/highlight"},
		{"@types/tmp@patch:@types/tmp@npm%3A0.1.0#builtin<compat/typescript>::version=0.1.0&hash=a45b0e", "@types/tmp"},
	}

	for _, testCase := range testCases {
		dependency := &YarnDependency{Value: testCase.dependencyValue}
		assert.Equal(t, testCase.expectedName, dependency.Name())
	}
}

func TestExtractAuthIdentFromNpmAuth(t *testing.T) {
	testCases := []struct {
		responseFromArtifactory string
		expectedExtractedAuth   string
	}{
		{"_auth = Z290Y2hhISB5b3UgcmVhbGx5IHRoaW5rIGkgd291bGQgcHV0IHJlYWwgY3JlZGVudGlhbHMgaGVyZT8=\nalways-auth = true\nemail = notexist@mail.com\n", "Z290Y2hhISB5b3UgcmVhbGx5IHRoaW5rIGkgd291bGQgcHV0IHJlYWwgY3JlZGVudGlhbHMgaGVyZT8="},
		{"always-auth=true\nemail=notexist@mail.com\n_auth=TGVhcCBhbmQgdGhlIHJlc3Qgd2lsbCBmb2xsb3c=\n", "TGVhcCBhbmQgdGhlIHJlc3Qgd2lsbCBmb2xsb3c="},
	}

	for _, testCase := range testCases {
		actualExtractedAuth, err := extractAuthIdentFromNpmAuth(testCase.responseFromArtifactory)
		assert.NoError(t, err)
		assert.Equal(t, testCase.expectedExtractedAuth, actualExtractedAuth)
	}
}

func TestGetYarnDependencyKeyFromLocator(t *testing.T) {
	testCases := []struct {
		yarnDepLocator string
		expectedDepKey string
	}{
		{"camelcase@npm:6.2.0", "camelcase@npm:6.2.0"},
		{"@babel/highlight@npm:7.14.0", "@babel/highlight@npm:7.14.0"},
		{"fsevents@patch:fsevents@npm%3A2.3.2#builtin<compat/fsevents>::version=2.3.2&hash=11e9ea", "fsevents@patch:fsevents@npm%3A2.3.2#builtin<compat/fsevents>::version=2.3.2&hash=11e9ea"},
		{"follow-redirects@virtual:c192f6b3b32cd5d11a443145a3883a70c04cbd7c813b53085dbaf50263735f1162f10fdbddd53c24e162ec3bc#npm:1.14.1", "follow-redirects@npm:1.14.1"},
	}

	for _, testCase := range testCases {
		assert.Equal(t, testCase.expectedDepKey, getYarnDependencyKeyFromLocator(testCase.yarnDepLocator))
	}
}

func TestAppendDependencyRecursively(t *testing.T) {
	dependenciesMap := map[string]*YarnDependency{
		// For test 1:
		"pack1@npm:1.0.0": {Value: "pack1@npm:1.0.0", Details: YarnDepDetails{Version: "1.0.0"}},
		"pack2@npm:1.0.0": {Value: "pack2@npm:1.0.0", Details: YarnDepDetails{Version: "1.0.0"}},
		"pack3@npm:1.0.0": {Value: "pack3@npm:1.0.0", Details: YarnDepDetails{Version: "1.0.0", Dependencies: []YarnDependencyPointer{{Locator: "pack1@virtual:c192f6b3b32cd5d11a443144e162ec3bc#npm:1.0.0"}, {Locator: "pack2@npm:1.0.0"}}}},
		// For test 2:
		"pack4@npm:1.0.0": {Value: "pack4@npm:1.0.0", Details: YarnDepDetails{Version: "1.0.0", Dependencies: []YarnDependencyPointer{{Locator: "pack5@npm:1.0.0"}}}},
		"pack5@npm:1.0.0": {Value: "pack5@npm:1.0.0", Details: YarnDepDetails{Version: "1.0.0", Dependencies: []YarnDependencyPointer{{Locator: "pack6@npm:1.0.0"}}}},
		"pack6@npm:1.0.0": {Value: "pack6@npm:1.0.0", Details: YarnDepDetails{Version: "1.0.0", Dependencies: []YarnDependencyPointer{{Locator: "pack4@npm:1.0.0"}}}},
	}
	// Build a previousBuildDependencies map to avoid fetching checksums from Artifactory
	prevDependency := &buildinfo.Dependency{Checksum: &buildinfo.Checksum{}}
	previousBuildDependencies := map[string]*buildinfo.Dependency{"pack1:1.0.0": prevDependency, "pack2:1.0.0": prevDependency, "pack3:1.0.0": prevDependency, "pack4:1.0.0": prevDependency, "pack5:1.0.0": prevDependency, "pack6:1.0.0": prevDependency}
	yarnCmd := &YarnCommand{dependencies: make(map[string]*buildinfo.Dependency)}

	testCases := []struct {
		dependency           *YarnDependency
		expectedDependencies map[string]*buildinfo.Dependency
	}{
		{
			dependenciesMap["pack3@npm:1.0.0"],
			map[string]*buildinfo.Dependency{
				"pack1:1.0.0": {Id: "pack1:1.0.0", Checksum: &buildinfo.Checksum{}, RequestedBy: [][]string{{"pack3:1.0.0", "rootpack:1.0.0"}}},
				"pack2:1.0.0": {Id: "pack2:1.0.0", Checksum: &buildinfo.Checksum{}, RequestedBy: [][]string{{"pack3:1.0.0", "rootpack:1.0.0"}}},
				"pack3:1.0.0": {Id: "pack3:1.0.0", Checksum: &buildinfo.Checksum{}, RequestedBy: [][]string{{"rootpack:1.0.0"}}},
			},
		}, {
			dependenciesMap["pack6@npm:1.0.0"],
			map[string]*buildinfo.Dependency{
				"pack4:1.0.0": {Id: "pack4:1.0.0", Checksum: &buildinfo.Checksum{}, RequestedBy: [][]string{{"pack6:1.0.0", "rootpack:1.0.0"}}},
				"pack5:1.0.0": {Id: "pack5:1.0.0", Checksum: &buildinfo.Checksum{}, RequestedBy: [][]string{{"pack4:1.0.0", "pack6:1.0.0", "rootpack:1.0.0"}}},
				"pack6:1.0.0": {Id: "pack6:1.0.0", Checksum: &buildinfo.Checksum{}, RequestedBy: [][]string{{"rootpack:1.0.0"}}},
			},
		},
	}

	for _, testCase := range testCases {
		producerConsumer := parallel.NewBounedRunner(1, false)
		errorsQueue := clientutils.NewErrorsQueue(1)
		yarnCmd.dependencies = make(map[string]*buildinfo.Dependency)
		go func() {
			defer producerConsumer.Done()
			err := yarnCmd.appendDependencyRecursively(testCase.dependency, []string{"rootpack:1.0.0"}, dependenciesMap, previousBuildDependencies, nil, producerConsumer, errorsQueue)
			assert.NoError(t, err)
		}()
		producerConsumer.Run()
		assert.True(t, reflect.DeepEqual(testCase.expectedDependencies, yarnCmd.dependencies), "The result dependencies tree doesn't match the expected. expected: %s, actual: %s", testCase.expectedDependencies, yarnCmd.dependencies)
	}
}
