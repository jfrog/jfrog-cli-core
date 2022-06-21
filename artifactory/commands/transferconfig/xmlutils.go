package transferconfig

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

func fixConfigXml(configXml, sourceBaseUrl, targetBaseUrl string) (string, error) {
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

func findAllXmlTagIndices(content, tagName string, ensureSingle bool) ([][]int, bool, error) {
	tagPattern, err := regexp.Compile(fmt.Sprintf(`<%s>([\s\S]*?)</%s>`, tagName, tagName))
	if err != nil {
		return nil, false, err
	}
	xmlTagIndices := tagPattern.FindAllStringSubmatchIndex(content, -1)
	if ensureSingle {
		if err := ensureSingleTag(tagName, xmlTagIndices); err != nil {
			return nil, false, err
		}
	}

	return xmlTagIndices, len(xmlTagIndices) > 0, nil
}

func splitXmlTag(content string, indices [][]int, currentIndex int) (prefix, inner, suffix string) {
	if currentIndex == 0 {
		prefix = content[:indices[0][2]]
	} else {
		prefix = content[indices[currentIndex][0]:indices[currentIndex][2]]
	}
	if currentIndex < len(indices)-1 {
		suffix = content[indices[currentIndex][3]:indices[currentIndex+1][0]]
	} else {
		suffix = content[indices[currentIndex][3]:]
	}
	return prefix, content[indices[currentIndex][2]:indices[currentIndex][3]], suffix
}

func ensureSingleTag(tagName string, xmlTagIndices [][]int) error {
	if len(xmlTagIndices) == 0 {
		log.Debug("No <" + tagName + "> were found in source artifactory.config.xml.")
		// No federated repositories
		return nil
	}
	if len(xmlTagIndices) > 1 {
		return errorutils.CheckErrorf("Found multiple <%s> entities in source artifactory.config.xml", tagName)
	}
	return nil
}
