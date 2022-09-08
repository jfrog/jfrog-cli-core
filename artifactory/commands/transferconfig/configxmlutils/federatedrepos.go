package configxmlutils

import "github.com/jfrog/jfrog-client-go/utils/log"

// Remove federated members of federated repositories in artifactory.config.xml
// configXml     - artifactory.config.xml of the source Artifactory
// targetBaseUrl - Base URL of the target Artifactory
func RemoveFederatedMembers(configXml string) (string, error) {
	xmlTagIndices, exist, err := findAllXmlTagIndices(configXml, `federatedRepositories`, true)
	if err != nil {
		return "", err
	}
	if !exist {
		return configXml, nil
	}
	results := ""
	prefix, content, suffix := splitXmlTag(configXml, xmlTagIndices, 0)
	federatedRepositories, err := fixFederatedRepositories(content)
	if err != nil {
		return "", err
	}
	results += prefix + federatedRepositories + suffix
	return results, nil
}

func fixFederatedRepositories(content string) (string, error) {
	xmlTagIndices, exist, err := findAllXmlTagIndices(content, `federatedRepository`, false)
	if err != nil {
		return "", err
	}
	if !exist {
		return content, nil
	}
	federatedMembersRemoved := false
	results := ""
	for i := range xmlTagIndices {
		prefix, content, suffix := splitXmlTag(content, xmlTagIndices, i)
		federatedRepository, removed, err := removeFederatedMembers(content)
		if err != nil {
			return "", err
		}
		results += prefix + federatedRepository + suffix
		federatedMembersRemoved = federatedMembersRemoved || removed
	}
	if federatedMembersRemoved {
		log.Info("☝️  All federated members have been excluded from your federated repositories. Please configure your federation in the target server repositories.")
	}
	return results, nil
}

func removeFederatedMembers(content string) (string, bool, error) {
	xmlTagIndices, exist, err := findAllXmlTagIndices(content, `federatedMembers`, false)
	if err != nil {
		return "", false, err
	}
	if !exist {
		return content, false, nil
	}
	prefix, _, suffix := splitXmlTag(content, xmlTagIndices, 0)
	return prefix + suffix, true, nil
}
