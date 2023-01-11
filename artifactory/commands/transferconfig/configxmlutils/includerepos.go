package configxmlutils

import (
	"encoding/xml"
	"fmt"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
)

// Remove non-included repositories from artifactory.config.xml
// configXml            - artifactory.config.xml of the source Artifactory
// includedRepositories - Selected repositories
func RemoveNonIncludedRepositories(configXml string, includeExcludeFilter *utils.IncludeExcludeFilter) (string, error) {
	for _, repoType := range append(utils.RepoTypes, "releaseBundles") {
		xmlTagIndices, exist, err := findAllXmlTagIndices(configXml, repoType+`Repositories`, true)
		if err != nil {
			return "", err
		}
		if !exist {
			continue
		}
		prefix, content, suffix := splitXmlTag(configXml, xmlTagIndices, 0)
		includedRepositories, err := doRemoveNonIncludedRepositories(content, repoType, includeExcludeFilter)
		if err != nil {
			return "", err
		}
		configXml = prefix + includedRepositories + suffix
	}
	return configXml, nil
}

func doRemoveNonIncludedRepositories(content string, repoType string, includeExcludeFilter *utils.IncludeExcludeFilter) (string, error) {
	xmlTagIndices, exist, err := findAllXmlTagIndices(content, repoType+`Repository`, false)
	if err != nil {
		return "", err
	}
	if !exist {
		return content, nil
	}
	results := ""
	for i := range xmlTagIndices {
		prefix, content, suffix := splitXmlTag(content, xmlTagIndices, i)
		shouldFilter, err := shouldRemoveRepository(content, includeExcludeFilter)
		if err != nil {
			return "", err
		}
		if !shouldFilter {
			results += prefix + content + suffix
		}
	}
	return results, nil
}

func shouldRemoveRepository(content string, includeExcludeFilter *utils.IncludeExcludeFilter) (bool, error) {
	rtRepo := &artifactoryRepository{}
	// The content of the repository tag must be wrapped inside an outer tag in order to be unmarshalled.
	content = fmt.Sprintf("<repo>%s</repo>", content)
	err := xml.Unmarshal([]byte(content), rtRepo)
	if err != nil {
		return false, err
	}

	includeRepo, err := includeExcludeFilter.ShouldIncludeRepository(rtRepo.Key)
	if err != nil {
		return false, err
	}
	return !includeRepo, nil
}

type artifactoryRepository struct {
	Key  string `xml:"key"`
	Type string `xml:"type"`
}
