package configxmlutils

import (
	"strings"
)

// Replace URL of federated repositories in artifactory.config.xml
// configXml     - artifactory.config.xml of the source Artifactory
// sourceBaseUrl - Base URL of the source Artifactory
// targetBaseUrl - Base URL of the target Artifactory
func ReplaceUrlsInFederatedrepos(configXml, sourceBaseUrl, targetBaseUrl string) (string, error) {
	sourceBaseUrl = strings.TrimSuffix(sourceBaseUrl, "/")
	targetBaseUrl = strings.TrimSuffix(targetBaseUrl, "/")

	xmlTagIndices, exist, err := findAllXmlTagIndices(configXml, `federatedRepositories`, true)
	if err != nil {
		return "", err
	}
	if !exist {
		return configXml, nil
	}
	results := ""
	prefix, content, suffix := splitXmlTag(configXml, xmlTagIndices, 0)
	federatedRepositories, err := fixFederatedRepositories(content, sourceBaseUrl, targetBaseUrl)
	if err != nil {
		return "", err
	}
	results += prefix + federatedRepositories + suffix
	return results, nil
}

func fixFederatedRepositories(content, sourceBaseUrl, targetBaseUrl string) (string, error) {
	xmlTagIndices, exist, err := findAllXmlTagIndices(content, `federatedRepository`, false)
	if err != nil {
		return "", err
	}
	if !exist {
		return content, nil
	}
	results := ""
	for i := range xmlTagIndices {
		prefix, content, suffix := splitXmlTag(content, xmlTagIndices, i)
		federatedRepository, err := fixFederatedRepository(content, sourceBaseUrl, targetBaseUrl)
		if err != nil {
			return "", err
		}
		results += prefix + federatedRepository + suffix
	}
	return results, nil
}

func fixFederatedRepository(content, sourceBaseUrl, targetBaseUrl string) (string, error) {
	xmlTagIndices, exist, err := findAllXmlTagIndices(content, `federatedMembers`, false)
	if err != nil {
		return "", err
	}
	if !exist {
		return content, nil
	}
	results := ""
	for i := range xmlTagIndices {
		prefix, content, suffix := splitXmlTag(content, xmlTagIndices, i)
		federatedMembers, err := fixFederatedMembers(content, sourceBaseUrl, targetBaseUrl)
		if err != nil {
			return "", err
		}
		results += prefix + federatedMembers + suffix
	}
	return results, nil
}

func fixFederatedMembers(content, sourceBaseUrl, targetBaseUrl string) (string, error) {
	xmlTagIndices, exist, err := findAllXmlTagIndices(content, `federatedMember`, false)
	if err != nil {
		return "", err
	}
	if !exist {
		return content, nil
	}
	results := ""
	for i := range xmlTagIndices {
		prefix, content, suffix := splitXmlTag(content, xmlTagIndices, i)
		federatedMember, err := fixFederatedMemberUrl(content, sourceBaseUrl, targetBaseUrl)
		if err != nil {
			return "", err
		}
		results += prefix + federatedMember + suffix
	}
	return results, nil
}

func fixFederatedMemberUrl(content, sourceBaseUrl, targetBaseUrl string) (string, error) {
	xmlTagIndices, exist, err := findAllXmlTagIndices(content, `url`, true)
	if err != nil {
		return "", err
	}
	if !exist {
		return content, nil
	}

	prefix, content, suffix := splitXmlTag(content, xmlTagIndices, 0)
	url := strings.TrimSpace(content)
	if strings.HasPrefix(url, sourceBaseUrl) {
		return prefix + strings.Replace(url, sourceBaseUrl, targetBaseUrl, 1) + suffix, nil
	}
	return prefix + content + suffix, nil
}
