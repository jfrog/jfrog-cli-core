package cisetup

import (
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
)

const ConfigServerId = "jfrog-instance"

type CiSetupData struct {
	RepositoryName string
	ProjectDomain  string
	VcsBaseUrl     string
	LocalDirPath   string
	GitBranch      string
	BuildName      string
	CiType         CiType
	// A collection of the technologies that were detected in the project.
	DetectedTechnologies map[coreutils.Technology]bool
	// The chosen build technology stored with all the necessary information.
	BuiltTechnology *TechnologyInfo
	VcsCredentials  VcsServerDetails
	GitProvider     GitProvider
}

type TechnologyInfo struct {
	Type               coreutils.Technology
	VirtualRepo        string
	LocalSnapshotsRepo string
	LocalReleasesRepo  string
	BuildCmd           string
}

func (sd *CiSetupData) GetRepoFullName() string {
	return sd.ProjectDomain + "/" + sd.RepositoryName
}

// Trim technology name from command prefix. (example: mvn clean install >> clean install)
func (sd *CiSetupData) GetBuildCmdForNativeStep() string {
	// Remove exec name.
	if sd.BuiltTechnology.Type.IsCiSetup() {
		return strings.TrimPrefix(strings.TrimSpace(sd.BuiltTechnology.BuildCmd), sd.BuiltTechnology.Type.GetExecCommandName()+" ")
	}

	return strings.TrimSpace(sd.BuiltTechnology.BuildCmd)
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
