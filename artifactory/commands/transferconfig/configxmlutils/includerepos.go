package configxmlutils

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
)

// Remove non-included repositories from artifactory.config.xml
// configXml            - artifactory.config.xml of the source Artifactory
// includedRepositories - Selected repositories
func FilterNonIncludedRepositories(configXml string, includedRepositories []string) (string, error) {
	for _, repoType := range []utils.RepoType{utils.LOCAL, utils.REMOTE, utils.VIRTUAL, utils.FEDERATED} {
		xmlTagIndices, exist, err := findAllXmlTagIndices(configXml, repoType.String()+`Repositories`, true)
		if err != nil {
			return "", err
		}
		if !exist {
			continue
		}
		prefix, content, suffix := splitXmlTag(configXml, xmlTagIndices, 0)
		includedRepositories, err := filterNonIncludedRepositories(content, repoType, includedRepositories)
		if err != nil {
			return "", err
		}
		configXml = prefix + includedRepositories + suffix
	}
	return configXml, nil
}

func filterNonIncludedRepositories(content string, repoType utils.RepoType, includedRepositories []string) (string, error) {
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
		shouldFilter, err := shouldFilterRepository(content, includedRepositories)
		if err != nil {
			return "", err
		}
		if !shouldFilter {
			results += prefix + content + suffix
		}
	}
	return results, nil
}

func shouldFilterRepository(content string, includedRepositories []string) (bool, error) {
	xmlTagIndices, _, err := findAllXmlTagIndices(content, `type`, true)
	if err != nil {
		return false, err
	}
	_, repositoryType, _ := splitXmlTag(content, xmlTagIndices, 0)
	if repositoryType == "buildinfo" {
		return false, nil
	}

	xmlTagIndices, _, err = findAllXmlTagIndices(content, `key`, true)
	if err != nil {
		return false, err
	}
	_, repositoryKey, _ := splitXmlTag(content, xmlTagIndices, 0)

	return !coreutils.Contains(includedRepositories, repositoryKey), nil
}
