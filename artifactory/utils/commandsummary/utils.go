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
	rtUrl := GetPlatformUrl()
	majorVersion := GetPlatformMajorVersion()
	if majorVersion == 6 {
		return fmt.Sprintf(artifactory6UiFormat, rtUrl, pathInRt)
	}
	return fmt.Sprintf(artifactory7UiFormat, rtUrl, pathInRt)
}

func WrapCollapsableMarkdown(title, markdown string) (string, error) {
	return fmt.Sprintf("\n\n\n<details open>\n\n<summary> <h4> %s </h4></summary><p></p>\n\n%s\n\n</details>\n\n\n", title, markdown), nil
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

// notScanned is a default implementation of the ScanResult interface.
type notScanned struct {
	Violations      string
	Vulnerabilities string
}

func (m *notScanned) GetViolations() string {
	return m.Violations
}

func (m *notScanned) GetVulnerabilities() string {
	return m.Vulnerabilities
}
