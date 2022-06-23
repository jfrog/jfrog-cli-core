package transferconfig

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// Needed files for config import in SaaS
var neededFiles = []string{
	"artifactory.properties",
	filepath.Join("etc", "access.bootstrap.json"),
	filepath.Join("etc", "security", "artifactory.key"),
	filepath.Join("etc", "security", "url.signing.key"),
}

// Archive the exportPath directory and the input artifactory.config.xml.
func archiveConfig(exportPath string, configXml string) (buffer *bytes.Buffer, retErr error) {
	buffer = &bytes.Buffer{}
	writer := zip.NewWriter(buffer)
	writer.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestCompression)
	})
	defer func() {
		closeErr := writer.Close()
		if retErr == nil {
			retErr = errorutils.CheckError(closeErr)
		}
	}()

	for _, neededFile := range neededFiles {
		neededFilePath := filepath.Join(exportPath, neededFile)
		log.Debug("Archiving " + neededFile)
		fileContent, err := os.ReadFile(neededFilePath)
		if err != nil {
			if os.IsNotExist(err) && strings.HasSuffix(neededFile, "url.signing.key") {
				log.Info("url.signing.key file is missing in the source Artifactory server. Skipping...")
				continue
			}
			return nil, errorutils.CheckError(err)
		}
		fileWriter, err := writer.Create(neededFile)
		if err != nil {
			return nil, errorutils.CheckError(err)
		}
		if _, retErr = fileWriter.Write(fileContent); errorutils.CheckError(retErr) != nil {
			return
		}
	}
	fileWriter, err := writer.Create("artifactory.config.xml")
	if err != nil {
		return buffer, errorutils.CheckError(err)
	}
	_, retErr = fileWriter.Write([]byte(configXml))
	return
}

func createArtifactoryClientDetails(serviceManager artifactory.ArtifactoryServicesManager) (*httputils.HttpClientDetails, error) {
	config := serviceManager.GetConfig()
	if config == nil {
		return nil, errorutils.CheckErrorf("expected full config, but no configuration exists")
	}
	rtDetails := config.GetServiceDetails()
	if rtDetails == nil {
		return nil, errorutils.CheckErrorf("artifactory details not configured")
	}
	clientDetails := rtDetails.CreateHttpClientDetails()
	return &clientDetails, nil
}
