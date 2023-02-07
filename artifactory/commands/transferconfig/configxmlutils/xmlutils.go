package configxmlutils

import (
	"fmt"
	"regexp"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// Remove all repositories from the config XML.
// Since Artifactory 7.49, we transfer the repositories using GET and POST to /api/repository.
// Therefore we don't need to have then in the config.xml.
func RemoveAllRepositories(configXml string) (string, error) {
	for _, repoType := range append(utils.RepoTypes, "releaseBundles") {
		xmlTagIndices, exist, err := findAllXmlTagIndices(configXml, repoType+`Repositories`, true)
		if err != nil {
			return "", err
		}
		if !exist {
			continue
		}
		prefix, _, suffix := splitXmlTag(configXml, xmlTagIndices, 0)
		configXml = prefix + suffix
	}
	return configXml, nil
}

// Find all indiced of the input tagName in the XML.
// xmlContent   - XML content
// tagName      - Tag name in XML
// ensureSingle - Make this function returns an error if more than 1 appearance of the tag detected
// Return values:
// indices - The 'i' represents appearance of a single tag and the 'j' represents the 4 indices of the start and end positions of the tag.
// exist   - True if the tag exist in the XML
// err     - Not nil if ensureSingle=true and there are many positions of the tag, or in case of an unexpected error
func findAllXmlTagIndices(xmlContent, tagName string, ensureSingle bool) (indices [][]int, exist bool, err error) {
	tagPattern, err := regexp.Compile(fmt.Sprintf(`<%s>([\s\S]*?)</%s>`, tagName, tagName))
	if err != nil {
		return
	}
	indices = tagPattern.FindAllStringSubmatchIndex(xmlContent, -1)
	if ensureSingle {
		if err = ensureSingleTag(tagName, indices); err != nil {
			return
		}
	}

	return indices, len(indices) > 0, nil
}

// Split the content into prefix, inner and suffix according to the output of findAllXmlTagIndices.
// xmlContent   - The XML content
// indices      - Indices of a tag in the XML (output of findAllXmlTagIndices)
// currentIndex - The current index of the tag
func splitXmlTag(xmlContent string, indices [][]int, currentIndex int) (prefix, inner, suffix string) {
	if currentIndex == 0 {
		prefix = xmlContent[:indices[0][2]]
	} else {
		prefix = xmlContent[indices[currentIndex][0]:indices[currentIndex][2]]
	}
	if currentIndex < len(indices)-1 {
		suffix = xmlContent[indices[currentIndex][3]:indices[currentIndex+1][0]]
	} else {
		suffix = xmlContent[indices[currentIndex][3]:]
	}
	return prefix, xmlContent[indices[currentIndex][2]:indices[currentIndex][3]], suffix
}

// Return error if the number of appearances of the tag in an XML is > 1.
func ensureSingleTag(tagName string, xmlTagIndices [][]int) error {
	if len(xmlTagIndices) == 0 {
		log.Debug("No <" + tagName + "> were found in source artifactory.config.xml.")
		// The tag was not found in the XML
		return nil
	}
	if len(xmlTagIndices) > 1 {
		return errorutils.CheckErrorf("Found multiple <%s> entities in source artifactory.config.xml", tagName)
	}
	return nil
}
