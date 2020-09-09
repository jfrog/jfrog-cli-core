package buildinfo

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/utils/log"
	"github.com/jfrog/jfrog-cli-core/utils/tests"
	"github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	withGit    = "git_test_.git_suffix"
	withoutGit = "git_test_no_.git_suffix"
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
	checkFailureAndClean(t, buildDir, dotGitPath)
	partials := getBuildInfoPartials(baseDir, t, buildName, "1")
	checkFailureAndClean(t, buildDir, dotGitPath)
	checkVCSUrl(partials, t)
	tests.RemovePath(buildDir, t)
	tests.RenamePath(dotGitPath, filepath.Join(filepath.Join("..", "testdata"), originalDir), t)
}

// Clean the environment if fails
func checkFailureAndClean(t *testing.T, buildDir string, oldPath string) {
	if t.Failed() {
		t.Log("Performing cleanup...")
		tests.RemovePath(buildDir, t)
		tests.RenamePath(oldPath, filepath.Join(filepath.Join("..", "testdata"), withGit), t)
		t.FailNow()
	}
}

func getBuildInfoPartials(baseDir string, t *testing.T, buildName string, buildNumber string) buildinfo.Partials {
	buildAddGitConfiguration := new(BuildAddGitCommand).SetDotGitPath(baseDir).SetBuildConfiguration(&utils.BuildConfiguration{BuildName: buildName, BuildNumber: buildNumber})
	err := buildAddGitConfiguration.Run()
	if err != nil {
		t.Error("Cannot run build add git due to: " + err.Error())
		return nil
	}
	partials, err := utils.ReadPartialBuildInfoFiles(buildName, buildNumber)
	if err != nil {
		t.Error("Cannot read partial build info due to: " + err.Error())
		return nil
	}
	return partials
}

func getBuildDir(t *testing.T) string {
	buildDir, err := utils.GetBuildDir(buildName, "1")
	if err != nil {
		t.Error("Cannot create temp dir due to: " + err.Error())
		return ""
	}
	return buildDir
}

func checkVCSUrl(partials buildinfo.Partials, t *testing.T) {
	for _, partial := range partials {
		if partial.Vcs != nil {
			url := partial.Vcs.Url
			urlSplitted := strings.Split(url, ".git")
			if len(urlSplitted) != 2 {
				t.Error("Argumanets value is different then two: ", urlSplitted)
				break
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

	// Clean git path
	tests.RenamePath(dotGitPath, filepath.Join(baseDir, originalFolder), t)
}

func TestRtDetailsFromConfigFile(t *testing.T) {
	expectedUrl := "http://localhost:8081/artifactory/"
	expectedUser := "admin"

	homeEnv := os.Getenv(coreutils.HomeDir)
	if homeEnv == "" {
		homeEnv = os.Getenv(coreutils.JfrogHomeEnv)
	}
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
	details, err := config.RtDetails()
	if err != nil {
		t.Error(err)
	}

	if details.Url != expectedUrl {
		t.Error(fmt.Sprintf("Expected %s, got %s", expectedUrl, details.Url))
	}
	if details.User != expectedUser {
		t.Error(fmt.Sprintf("Expected %s, got %s", details.User, expectedUser))
	}
}

func TestRtDetailsWithoutConfigFile(t *testing.T) {
	expectedUrl := "http://localhost:8082/artifactory/"
	expectedUser := "admin2"

	homeEnv := os.Getenv(coreutils.HomeDir)
	if homeEnv == "" {
		homeEnv = os.Getenv(coreutils.JfrogHomeEnv)
	}
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
	details, err := config.RtDetails()
	if err != nil {
		t.Error(err)
	}

	if details.Url != expectedUrl {
		t.Error(fmt.Sprintf("Expected %s, got %s", expectedUrl, details.Url))
	}

	if details.User != expectedUser {
		t.Error(fmt.Sprintf("Expected %s, got %s", details.User, expectedUser))
	}
}
