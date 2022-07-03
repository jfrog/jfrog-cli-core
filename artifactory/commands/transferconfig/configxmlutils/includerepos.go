package configxmlutils

import (
	"encoding/xml"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
)

// Remove non-included repositories from artifactory.config.xml
// configXml            - artifactory.config.xml of the source Artifactory
// includedRepositories - Selected repositories
func RemoveNonIncludedRepositories(configXml string, includedRepositories []string) (string, error) {
	for _, repoType := range []utils.RepoType{utils.LOCAL, utils.REMOTE, utils.VIRTUAL, utils.FEDERATED} {
		xmlTagIndices, exist, err := findAllXmlTagIndices(configXml, repoType.String()+`Repositories`, true)
		if err != nil {
			return "", err
		}
		if !exist {
			continue
		}
		prefix, content, suffix := splitXmlTag(configXml, xmlTagIndices, 0)
		includedRepositories, err := doRemoveNonIncludedRepositories(content, repoType, includedRepositories)
		if err != nil {
			return "", err
		}
		configXml = prefix + includedRepositories + suffix
	}
	return configXml, nil
}

func doRemoveNonIncludedRepositories(content string, repoType utils.RepoType, includedRepositories []string) (string, error) {
	xmlTagIndices, exist, err := findAllXmlTagIndices(content, repoType.String()+`Repository`, false)
	if err != nil {
		return "", err
	}
	if !exist {
		return content, nil
	}
	results := ""
	for i := range xmlTagIndices {
		prefix, content, suffix := splitXmlTag(content, xmlTagIndices, i)
		shouldFilter, err := shouldRemoveRepository(content, includedRepositories)
		if err != nil {
			return "", err
		}
		if !shouldFilter {
			results += prefix + content + suffix
		}
	}
	return results, nil
}

func shouldRemoveRepository(content string, includedRepositories []string) (bool, error) {
	rtRepo := &artifactoryRepository{}
	// The content of the repository tag must be wrapped inside an outer tag in order to be unmarshalled.
	content = "<repo>" + content + "</repo>"
	err := xml.Unmarshal([]byte(content), rtRepo)
	if err != nil {
		return false, err
	}
	if rtRepo.Type == "buildinfo" {
		return false, nil
	}
	return !coreutils.Contains(includedRepositories, rtRepo.Key), nil
}

type artifactoryRepository struct {
	Key  string `xml:"key"`
	Type string `xml:"type"`
}
