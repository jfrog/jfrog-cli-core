package configxmlutils

// Remove federated members of federated repositories in artifactory.config.xml
// configXml - artifactory.config.xml of the source Artifactory
// Return the modified config.xml, a boolean indicating whether federated repositories were actually removed and an error, if any.
func RemoveFederatedMembers(configXml string) (string, bool, error) {
	xmlTagIndices, exist, err := findAllXmlTagIndices(configXml, `federatedRepositories`, true)
	if err != nil {
		return "", false, err
	}
	if !exist {
		return configXml, false, nil
	}
	results := ""
	prefix, content, suffix := splitXmlTag(configXml, xmlTagIndices, 0)
	federatedRepositories, federatedMembersRemoved, err := fixFederatedRepositories(content)
	if err != nil {
		return "", false, err
	}
	results += prefix + federatedRepositories + suffix
	return results, federatedMembersRemoved, nil
}

// Remove federated members of federated repositories in all federated repositories.
// content - The federated repositories content in artifactory.config.xml
// Return the modified federated repositories in the config.xml, a boolean indicating whether federated repositories were actually removed and an error, if any.
func fixFederatedRepositories(content string) (string, bool, error) {
	xmlTagIndices, exist, err := findAllXmlTagIndices(content, `federatedRepository`, false)
	if err != nil {
		return "", false, err
	}
	if !exist {
		return content, false, nil
	}
	// federatedMembersRemoved indicates whether at least one federated member is removed
	federatedMembersRemoved := false
	results := ""
	for i := range xmlTagIndices {
		prefix, content, suffix := splitXmlTag(content, xmlTagIndices, i)
		federatedRepository, removed, err := removeFederatedMembers(content)
		if err != nil {
			return "", false, err
		}
		results += prefix + federatedRepository + suffix
		// federatedMembersRemoved will be true only if at least one of the "removed" values will be true
		federatedMembersRemoved = federatedMembersRemoved || removed
	}
	return results, federatedMembersRemoved, nil
}

// Remove federated members of federated repositories in a federated repository.
// content - The federated repository content in artifactory.config.xml
// Return the modified federated repository in the config.xml, a boolean indicating whether federated repositories were actually removed and an error, if any.
func removeFederatedMembers(content string) (string, bool, error) {
	xmlTagIndices, exist, err := findAllXmlTagIndices(content, `federatedMembers`, false)
	if err != nil {
		return "", false, err
	}
	if !exist {
		return content, false, nil
	}
	// The actual removing of the federated members content - We ignore the inner content of the XML tag.
	prefix, _, suffix := splitXmlTag(content, xmlTagIndices, 0)
	return prefix + suffix, true, nil
}
