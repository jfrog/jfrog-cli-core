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

// Map containing indexed data recorded to the file system.
// The key is the index and the value is a map of file names as SHA1 to their full path.
type IndexedFilesMap map[Index]map[string]string

// Receives an index and a predicted file name, return the value if exists.
func (nm IndexedFilesMap) Get(index Index, key string) (exists bool, value string) {
	if _, exists := nm[index]; exists {
		shaKey := fileNameToSha1(key)
		if _, exists := nm[index][shaKey]; exists {
			return true, nm[index][shaKey]
		}
	}
	return
}

func fileNameToSha1(fileName string) string {
	hash := sha1.New() // #nosec G401 - This is only used for encoding, not security.
	hash.Write([]byte(fileName))
	hashBytes := hash.Sum(nil)
	return hex.EncodeToString(hashBytes)
}
