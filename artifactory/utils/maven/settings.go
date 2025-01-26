package maven

import (
	"encoding/xml"
	"github.com/apache/camel-k/v2/pkg/util/maven"
	"os"
)

// UpdateArtifactoryMirror updates or adds an Artifactory mirror in the settings.xml file.
// It preserves all other fields and ensures the settings remain valid.
func UpdateArtifactoryMirror(filePath, url string) error {
	// Read the existing settings file
	file, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var settings maven.Settings
	err = xml.Unmarshal(file, &settings)
	if err != nil {
		return err
	}

	// Update or add the Artifactory mirror
	updated := false
	for i, mirror := range settings.Mirrors {
		if mirror.ID == "artifactory" {
			settings.Mirrors[i].URL = url
			updated = true
			break
		}
	}

	if !updated {
		settings.Mirrors = append(settings.Mirrors, maven.Mirror{
			ID:       "artifactory",
			MirrorOf: "*",
			URL:      url,
		})
	}

	// Marshal the updated settings back to XML
	data, err := xml.MarshalIndent(&settings, "", "  ")
	if err != nil {
		return err
	}

	// Add XML header and write to file
	data = append([]byte(xml.Header), data...)
	return os.WriteFile(filePath, data, 0644)
}
