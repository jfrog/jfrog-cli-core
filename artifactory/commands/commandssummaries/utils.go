package commandssummaries

import (
	"fmt"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
)

const (
	artifactory7UiFormat = "%sui/repos/tree/General/%s?clearFilter=true"
	artifactory6UiFormat = "%sartifactory/webapp/#/artifacts/browse/tree/General/%s"

	artifactoryDockerPackagesUiFormat = "%s/ui/packages/docker:%s/sha256__%s"
)

func generateArtifactUrl(rtUrl, pathInRt string, majorVersion int) string {
	rtUrl = clientUtils.AddTrailingSlashIfNeeded(rtUrl)
	if majorVersion == 6 {
		return fmt.Sprintf(artifactory6UiFormat, rtUrl, pathInRt)
	}
	return fmt.Sprintf(artifactory7UiFormat, rtUrl, pathInRt)
}
