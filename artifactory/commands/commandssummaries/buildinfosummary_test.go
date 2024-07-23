package commandssummaries

import (
	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBuildInfoTable(t *testing.T) {
	gh := &BuildInfoSummary{}
	var builds = []*buildinfo.BuildInfo{
		{
			Name:     "buildName",
			Number:   "123",
			Started:  "2024-05-05T12:47:20.803+0300",
			BuildUrl: "http://myJFrogPlatform/builds/buildName/123",
		},
	}
	expected := "\n\n ### Published Builds  \n\n\n\n|  Build Info |  Time Stamp | \n|---------|------------| \n| [buildName 123](http://myJFrogPlatform/builds/buildName/123) | May 5, 2024 , 12:47:20 |\n\n\n"
	assert.Equal(t, expected, gh.buildInfoTable(builds))
}

// Validates a generic modules DOES NOT appear in the published modules
// This to avoid duplication with uploaded artifacts summary.
// Notice that python modules will be ignored, as they are labeled as generic.
func TestBuildInfoModules_GenericModuleIgnored(t *testing.T) {
	gh := &BuildInfoSummary{serverUrl: "https://myJFrogPlatform.io/"}
	builds := []*buildinfo.BuildInfo{
		{
			Modules: []buildinfo.Module{
				{
					Type: "generic",
					Id:   "generic-module",
					Artifacts: []buildinfo.Artifact{
						{Name: "generic-artifact-1.0.0.txt"},
					},
				},
			},
		},
	}
	expected := "\n\n ### Published Modules  \n\n"
	actual := gh.buildInfoModules(builds)
	assert.Equal(t, expected, actual)
}

// Validate an npm module is valid with a link
func TestBuildInfoModules_NonGenericModuleIncluded(t *testing.T) {
	gh := &BuildInfoSummary{serverUrl: "https://myJFrogPlatform.io/"}
	builds := []*buildinfo.BuildInfo{
		{
			Modules: []buildinfo.Module{
				{
					Type: "npm",
					Id:   "non-generic-module",
					Artifacts: []buildinfo.Artifact{
						{
							Name:         "npm-example-0.0.3.tgz",
							Path:         "npm-example/-/npm-example-0.0.3.tgz",
							OriginalRepo: "origin-repo",
						},
					},
				},
			},
		},
	}
	expected := "\n\n ### Published Modules  \n\n\n ### `non-generic-module` \n\n\n <pre>📦 non-generic-module\n└── <a href=https://myJFrogPlatform.io/ui/repos/tree/General/origin-repo/npm-example/-/npm-example-0.0.3.tgz target=\"_blank\">npm-example-0.0.3.tgz</a>\n\n</pre>"
	actual := gh.buildInfoModules(builds)
	assert.Equal(t, expected, actual)
}

// Validates that non-supported package managers, which don't have an original repo key yet,
// do not show links as they are not supported.
func TestBuildInfoModules_DontShowLinkIfOriginalRepoNotProvided(t *testing.T) {
	gh := &BuildInfoSummary{serverUrl: "https://myJFrogPlatform.io/"}
	builds := []*buildinfo.BuildInfo{
		{
			Modules: []buildinfo.Module{
				{
					Type: "docker",
					Id:   "i-dont-have-repo-key-yet",
					Artifacts: []buildinfo.Artifact{
						{
							Name: "artifact-1.0.0.tar.gz",
							Path: "a/b/artifact-1.0.0.tar.gz",
						},
					},
				},
			},
		},
	}
	expected := "\n\n ### Published Modules  \n\n\n ### `i-dont-have-repo-key-yet` \n\n\n <pre>📦 i-dont-have-repo-key-yet\n└── 📄 artifact-1.0.0.tar.gz\n\n</pre>"
	actual := gh.buildInfoModules(builds)
	assert.Equal(t, expected, actual)
}

// Validate an npm module is valid with a valid link
func TestBuildInfoModules_NpmModuleWithLink(t *testing.T) {
	gh := &BuildInfoSummary{serverUrl: "https://myJFrogPlatform.io/"}
	builds := []*buildinfo.BuildInfo{
		{
			Modules: []buildinfo.Module{
				{
					Type: "npm",
					Id:   "npm-module-with-link",
					Artifacts: []buildinfo.Artifact{
						{
							Name:         "npm-link-example-0.0.3.tgz",
							Path:         "npm-link-example/-/npm-example-0.0.3.tgz",
							OriginalRepo: "origin-repo",
						},
					},
				},
			},
		},
	}
	expected := "\n\n ### Published Modules  \n\n\n ### `npm-module-with-link` \n\n\n <pre>📦 npm-module-with-link\n└── <a href=https://myJFrogPlatform.io/ui/repos/tree/General/origin-repo/npm-link-example/-/npm-example-0.0.3.tgz target=\"_blank\">npm-link-example-0.0.3.tgz</a>\n\n</pre>"
	actual := gh.buildInfoModules(builds)
	assert.Equal(t, expected, actual)
}

func TestParseBuildTime(t *testing.T) {
	// Test format
	actual := parseBuildTime("2006-01-02T15:04:05.000-0700")
	expected := "Jan 2, 2006 , 15:04:05"
	assert.Equal(t, expected, actual)
	// Test invalid format
	expected = "N/A"
	actual = parseBuildTime("")
	assert.Equal(t, expected, actual)
}
