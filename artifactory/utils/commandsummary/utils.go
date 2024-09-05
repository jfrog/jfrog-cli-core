package commandsummary

import (
	"crypto/sha1" // #nosec G505 - This is only used for encoding, not security.
	"encoding/hex"
	"fmt"
)

const (
	artifactory7UiFormat              = "%sui/repos/tree/General/%s?clearFilter=true"
	artifactory6UiFormat              = "%sartifactory/webapp/#/artifacts/browse/tree/General/%s"
	artifactoryDockerPackagesUiFormat = "%s/ui/packages/docker:%s/sha256__%s"
)

func GenerateArtifactUrl(pathInRt string) string {
	if StaticMarkdownConfig.GetPlatformMajorVersion() == 6 {
		return fmt.Sprintf(artifactory6UiFormat, StaticMarkdownConfig.GetPlatformUrl(), pathInRt)
	}
	return fmt.Sprintf(artifactory7UiFormat, StaticMarkdownConfig.GetPlatformUrl(), pathInRt)
}

func WrapCollapsableMarkdown(title, markdown string, headerSize int) string {
	return fmt.Sprintf("\n\n\n<details open>\n\n<summary> <h%d> %s </h%d></summary><p></p>\n\n%s\n\n</details>\n\n\n", headerSize, title, headerSize, markdown)
}

// Map containing indexed data recorded to the file system.
// The key is the index and the value is a map of file names as SHA1 to their full path.
type IndexedFilesMap map[Index]map[string]string

func fileNameToSha1(fileName string) string {
	hash := sha1.New() // #nosec G401 - This is only used for encoding, not security.
	hash.Write([]byte(fileName))
	hashBytes := hash.Sum(nil)
	return hex.EncodeToString(hashBytes)
}
