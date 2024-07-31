package commandssummaries

import (
	"fmt"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
)

const (
	artifactory7UiFormat = "%sui/repos/tree/General/%s"
	artifactory6UiFormat = "%sartifactory/webapp/#/artifacts/browse/tree/General/%s"
)

func generateArtifactUrl(rtUrl, pathInRt, project string, majorVersion int) string {
	rtUrl = clientUtils.AddTrailingSlashIfNeeded(rtUrl)
	if majorVersion == 6 {
		return fmt.Sprintf(artifactory6UiFormat, rtUrl, pathInRt)
	}
	uri := fmt.Sprintf(artifactory7UiFormat, rtUrl, pathInRt)
	if project != "" {
		uri += "?projectKey=" + project
	}
	return uri
}
