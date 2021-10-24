package buildinfo

import (
	"fmt"
	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	testsutils "github.com/jfrog/jfrog-client-go/utils/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/log"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
)

const (
	withGit    = "git_test_.git_suffix"
	withoutGit = "git_test_no_.git_suffix"
	withBranch = "git_issues2_.git_suffix"
	buildName  = "TestExtractGitUrl"
)

func init() {
	log.SetDefaultLogger()
}

func TestExtractGitUrlWithDotGit(t *testing.T) {
	runTest(t, withGit)
}

func TestExtractGitUrlWithoutDotGit(t *testing.T) {
	runTest(t, withoutGit)
}

func runTest(t *testing.T, originalDir string) {
	baseDir, dotGitPath := tests.PrepareDotGitDir(t, originalDir, filepath.Join("..", "testdata"))
	buildDir := getBuildDir(t)
	defer cleanUp(t, buildDir, dotGitPath, originalDir)
	err := runBuildAddGit(t, buildName, "1", baseDir, true)
	if err != nil {
		return
	}
	partials := getBuildInfoPartials(t, buildName, "1", "")
	checkVCSUrl(partials, t)
}

func TestBuildAddGitSubmodules(t *testing.T) {
	var projectPath, tmpDir string
	projectPath, tmpDir = testsutils.InitVcsSubmoduleTestDir(t, filepath.Join("..", "testdata", "git_test_submodule"))
	defer fileutils.RemoveTempDir(tmpDir)

	testsName := []string{"dotGitProvided", "dotGitSearched"}
	for _, test := range testsName {
		t.Run(test, func(t *testing.T) {
			tmpBuildName := test + "-Build-" + strconv.FormatInt(time.Now().Unix(), 10)
			err := runBuildAddGit(t, tmpBuildName, "1", projectPath, test == "dotGitProvided")
			require.NoError(t, err)
			partials := getBuildInfoPartials(t, tmpBuildName, "1", "")
			assertVcsSubmodules(t, partials)
		})
	}
}

func TestBuildAddGitVCSDetails(t *testing.T) {
	bagTests := []struct {
		name        string
		originalDir string
		revision    string
		branch      string
		message     string
	}{
		{"Test vcs details without branch", withGit, "6198a6294722fdc75a570aac505784d2ec0d1818", "", "TEST-2 - Adding text to file1.txt"},
		{"Test vcs details with branch", withBranch, "b033a0e508bdb52eee25654c9e12db33ff01b8ff", "master", "TEST-4 - Adding text to file2.txt"}}

	for _, test := range bagTests {
		t.Run(test.name, func(t *testing.T) {
			baseDir, dotGitPath := tests.PrepareDotGitDir(t, test.originalDir, filepath.Join("..", "testdata"))
			buildDir := getBuildDir(t)
			defer cleanUp(t, buildDir, dotGitPath, test.originalDir)
			err := runBuildAddGit(t, buildName, "1", baseDir, true)
			if err != nil {
				return
			}
			partials := getBuildInfoPartials(t, buildName, "1", "")
			assertVCSDetails(partials, test.revision, test.branch, test.message, t)
		})
	}
}

func assertVCSDetails(partials buildinfo.Partials, revision, branch, message string, t *testing.T) {
	for _, partial := range partials {
		if partial.VcsList != nil {
			for _, vcs := range partial.VcsList {
				assert.Equal(t, revision, vcs.Revision)
				assert.Equal(t, branch, vcs.Branch)
				assert.Equal(t, message, vcs.Message)
			}
		} else {
			t.Error("VCS cannot be nil")
			break
		}
	}
}

func assertVcsSubmodules(t *testing.T, partials buildinfo.Partials) {
	assert.Len(t, partials, 1)
	vcsList := partials[0].VcsList
	assert.NotNil(t, vcsList)
	assert.Len(t, vcsList, 1)
	curVcs := vcsList[0]
	assert.Equal(t, "https://github.com/jfrog/jfrog-cli.git", curVcs.Url)
	assert.Equal(t, "6198a6294722fdc75a570aac505784d2ec0d1818", curVcs.Revision)
	assert.Equal(t, "submodule", curVcs.Branch)
	assert.Equal(t, "TEST-2 - Adding text to file1.txt", curVcs.Message)
}

func cleanUp(t *testing.T, buildDir, dotGitPath, originalDir string) {
	if buildDir != "" {
		tests.RemovePath(buildDir, t)
	}
	if dotGitPath != "" {
		tests.RenamePath(dotGitPath, filepath.Join("..", "testdata", originalDir), t)
	}
}

func getBuildInfoPartials(t *testing.T, buildName, buildNumber, projectKey string) buildinfo.Partials {
	partials, err := utils.ReadPartialBuildInfoFiles(buildName, buildNumber, projectKey)
	if err != nil {
		assert.NoError(t, err)
		return nil
	}
	return partials
}

// Run BAG command. If setDotGit==true, provide baseDir to the command. Else, change wd to baseDir and make the command find .git manually.
func runBuildAddGit(t *testing.T, buildName, buildNumber string, baseDir string, setDotGit bool) error {
	buildAddGitConfiguration := new(BuildAddGitCommand).SetBuildConfiguration(&utils.BuildConfiguration{BuildName: buildName, BuildNumber: buildNumber})
	if setDotGit {
		buildAddGitConfiguration.SetDotGitPath(baseDir)
	} else {
		wd, err := os.Getwd()
		if err != nil {
			assert.Error(t, err)
			return err
		}
		defer os.Chdir(wd)

		err = os.Chdir(baseDir)
		if err != nil {
			assert.Error(t, err)
			return err
		}
	}
	err := buildAddGitConfiguration.Run()
	assert.NoError(t, err)
	return err
}

func getBuildDir(t *testing.T) string {
	buildDir, err := utils.GetBuildDir(buildName, "1", "")
	if err != nil {
		t.Error("Cannot create temp dir due to: " + err.Error())
		return ""
	}
	return buildDir
}

func checkVCSUrl(partials buildinfo.Partials, t *testing.T) {
	for _, partial := range partials {
		if partial.VcsList != nil {
			for _, vcs := range partial.VcsList {
				url := vcs.Url
				urlSplitted := strings.Split(url, ".git")
				if len(urlSplitted) != 2 {
					t.Error("Arguments value is different than two: ", urlSplitted)
					break
				}
			}
		} else {
			t.Error("VCS cannot be nil")
			break
		}
	}
}

func TestPopulateIssuesConfigurations(t *testing.T) {
	// Test success scenario
	expectedIssuesConfiguration := &IssuesConfiguration{
		ServerID:          "local",
		TrackerName:       "TESTING",
		TrackerUrl:        "http://TESTING.com",
		Regexp:            `([a-zA-Z]+-[0-9]*)\s-\s(.*)`,
		KeyGroupIndex:     1,
		SummaryGroupIndex: 2,
		Aggregate:         true,
		AggregationStatus: "RELEASE",
		LogLimit:          100,
	}
	ic := new(IssuesConfiguration)
	// Build config from file
	err := ic.populateIssuesConfigsFromSpec(filepath.Join("..", "testdata", "buildissues", "issuesconfig_success.yaml"))
	// Check they are equal
	if err != nil {
		t.Error(fmt.Sprintf("Reading configurations file ended with error: %s", err.Error()))
		t.FailNow()
	}
	if *ic != *expectedIssuesConfiguration {
		t.Error(fmt.Sprintf("Failed reading configurations file. Expected: %+v Received: %+v", *expectedIssuesConfiguration, *ic))
		t.FailNow()
	}

	// Test failing scenarios
	failing := []string{
		filepath.Join("..", "testdata", "buildissues", "issuesconfig_fail_no_issues.yaml"),
		filepath.Join("..", "testdata", "buildissues", "issuesconfig_fail_invalid_groupindex.yaml"),
		filepath.Join("..", "testdata", "buildissues", "issuesconfig_fail_invalid_aggregate.yaml"),
	}

	for _, config := range failing {
		err = ic.populateIssuesConfigsFromSpec(config)
		if err == nil {
			t.Error(fmt.Sprintf("Reading configurations file was supposed to end with error: %s", config))
			t.FailNow()
		}
	}
}

func TestAddGitDoCollect(t *testing.T) {
	// Create git folder with files
	originalFolder := "git_issues_.git_suffix"
	baseDir, dotGitPath := tests.PrepareDotGitDir(t, originalFolder, filepath.Join("..", "testdata"))

	// Create BuildAddGitCommand
	config := BuildAddGitCommand{
		issuesConfig: &IssuesConfiguration{
			LogLimit:          100,
			Aggregate:         false,
			SummaryGroupIndex: 2,
			KeyGroupIndex:     1,
			Regexp:            `(.+-[0-9]+)\s-\s(.+)`,
			TrackerName:       "test",
		},
		buildConfiguration: &utils.BuildConfiguration{BuildNumber: "1", BuildName: "cli-tests-rt-build1"},
		configFilePath:     "",
		dotGitPath:         dotGitPath,
	}

	// Collect issues
	issues, err := config.DoCollect(config.issuesConfig, "")
	if err != nil {
		t.Error(err)
	}
	if len(issues) != 2 {
		// Error - should be empty
		t.Errorf("Issues list expected to have 2 issues, instead found %d issues: %v", len(issues), issues)
	}

	// Clean previous git path
	tests.RenamePath(dotGitPath, filepath.Join(baseDir, originalFolder), t)
	// Check if needs to fail
	if t.Failed() {
		t.FailNow()
	}
	// Set new git path
	originalFolder = "git_issues2_.git_suffix"
	baseDir, dotGitPath = tests.PrepareDotGitDir(t, originalFolder, filepath.Join("..", "testdata"))

	// Collect issues - we pass a revision, so only 2 of the 4 existing issues should be collected
	issues, err = config.DoCollect(config.issuesConfig, "6198a6294722fdc75a570aac505784d2ec0d1818")
	if err != nil {
		t.Error(err)
	}
	if len(issues) != 2 {
		// Error - should find 2 issues
		t.Errorf("Issues list expected to have 2 issues, instead found %d issues: %v", len(issues), issues)
	}

	// Test collection with a made up revision - the command should not throw an error, and 0 issues should be returned.
	issues, err = config.DoCollect(config.issuesConfig, "abcdefABCDEF1234567890123456789012345678")
	assert.NoError(t, err)
	assert.Empty(t, issues)

	// Clean git path
	tests.RenamePath(dotGitPath, filepath.Join(baseDir, originalFolder), t)
}

func TestServerDetailsFromConfigFile(t *testing.T) {
	expectedUrl := "http://localhost:8081/artifactory/"
	expectedUser := "admin"

	homeEnv := os.Getenv(coreutils.HomeDir)
	defer os.Setenv(coreutils.HomeDir, homeEnv)
	baseDir, err := os.Getwd()
	if err != nil {
		t.Error(err)
	}
	err = os.Setenv(coreutils.HomeDir, filepath.Join(baseDir, "..", "testdata"))
	if err != nil {
		t.Error(err)
	}
	configFilePath := filepath.Join("..", "testdata", "buildissues", "issuesconfig_success.yaml")
	config := BuildAddGitCommand{
		configFilePath: configFilePath,
	}
	details, err := config.ServerDetails()
	if err != nil {
		t.Error(err)
	}

	if details.ArtifactoryUrl != expectedUrl {
		t.Error(fmt.Sprintf("Expected %s, got %s", expectedUrl, details.ArtifactoryUrl))
	}
	if details.User != expectedUser {
		t.Error(fmt.Sprintf("Expected %s, got %s", details.User, expectedUser))
	}
}

func TestServerDetailsWithoutConfigFile(t *testing.T) {
	expectedUrl := "http://localhost:8082/artifactory/"
	expectedUser := "admin2"

	homeEnv := os.Getenv(coreutils.HomeDir)
	defer os.Setenv(coreutils.HomeDir, homeEnv)

	baseDir, err := os.Getwd()
	if err != nil {
		t.Error(err)
	}
	err = os.Setenv(coreutils.HomeDir, filepath.Join(baseDir, "..", "testdata"))
	if err != nil {
		t.Error(err)
	}

	config := BuildAddGitCommand{}
	details, err := config.ServerDetails()
	if err != nil {
		t.Error(err)
	}

	if details.ArtifactoryUrl != expectedUrl {
		t.Error(fmt.Sprintf("Expected %s, got %s", expectedUrl, details.ArtifactoryUrl))
	}

	if details.User != expectedUser {
		t.Error(fmt.Sprintf("Expected %s, got %s", details.User, expectedUser))
	}
}
