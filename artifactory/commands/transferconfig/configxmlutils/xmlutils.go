package configxmlutils

import (
	"fmt"
	"regexp"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

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
