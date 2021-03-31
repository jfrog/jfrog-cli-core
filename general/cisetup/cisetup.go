package cisetup

const ConfigServerId = "ci-setup-cmd"

type CiSetupData struct {
	RepositoryName          string
	ProjectDomain           string
	LocalDirPath            string
	GitBranch               string
	BuildCommand            string
	BuildName               string
	ArtifactoryVirtualRepos map[Technology]string
	// A collection of technologies that was found with a list of theirs indications
	DetectedTechnologies map[Technology]bool
	VcsCredentials       VcsServerDetails
	GitProvider          GitProvider
}

func (sd *CiSetupData) GetRepoFullName() string {
	return sd.ProjectDomain + "/" + sd.RepositoryName
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
