package cisetup

import "strings"

const ConfigServerId = "ci-setup-cmd"

type CiSetupData struct {
	RepositoryName string
	ProjectDomain  string
	VcsBaseUrl     string
	LocalDirPath   string
	GitBranch      string
	BuildName      string
	CiType         CiType
	// A collection of the technologies that were detected in the project.
	DetectedTechnologies map[Technology]bool
	// The chosen build technology stored with all the necessary information.
	BuiltTechnology *TechnologyInfo
	VcsCredentials  VcsServerDetails
	GitProvider     GitProvider
}

type TechnologyInfo struct {
	Type               Technology
	VirtualRepo        string
	LocalSnapshotsRepo string
	LocalReleasesRepo  string
	BuildCmd           string
}

func (sd *CiSetupData) GetRepoFullName() string {
	return sd.ProjectDomain + "/" + sd.RepositoryName
}

func (sd *CiSetupData) GetBuildCmdForNativeStep() string {
	// Remove exec name.
	return strings.TrimPrefix(strings.TrimSpace(sd.BuiltTechnology.BuildCmd), execNames[sd.BuiltTechnology.Type]+" ")
}

type VcsServerDetails struct {
	Url         string `json:"url,omitempty"`
	User        string `json:"user,omitempty"`
	Password    string `json:"-"`
	AccessToken string `json:"-"`
}

type GitProvider string

const (
	Github           = "GitHub"
	GithubEnterprise = "GitHub Enterprise"
	Bitbucket        = "Bitbucket"
	BitbucketServer  = "Bitbucket Server"
	Gitlab           = "GitLab"
)

type CiType string

const (
	Jenkins       = "Jenkins"
	GithubActions = "GitHub Actions"
	Pipelines     = "JFrog Pipelines"
)

var execNames = map[Technology]string{
	Maven:  "mvn",
	Gradle: "gradle",
	Npm:    "npm",
}
