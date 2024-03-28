package transferconfig

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
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
		retErr = errors.Join(retErr, errorutils.CheckError(writer.Close()))
	}()

	err := handleTypoInAccessBootstrap(exportPath)
	if err != nil {
		return nil, err
	}

	for _, neededFile := range neededFiles {
		neededFilePath := filepath.Join(exportPath, neededFile)
		log.Debug("Archiving " + neededFile)
		fileContent, err := os.ReadFile(neededFilePath)
		if err != nil {
			if os.IsNotExist(err) && strings.Contains(neededFile, "security") {
				log.Info(neededFile + " file is missing in the source Artifactory server. Skipping...")
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

// In some versions of Artifactory, the file access.bootstrap.json has a typo in its name: access.boostrap.json.
// If this is the case, rename the file to the right name.
func handleTypoInAccessBootstrap(exportPath string) error {
	accessBootstrapPath := filepath.Join(exportPath, "etc", "access.bootstrap.json")
	accessBootstrapExists, err := fileutils.IsFileExists(accessBootstrapPath, false)
	if err != nil {
		return err
	}
	if !accessBootstrapExists {
		err = fileutils.MoveFile(filepath.Join(exportPath, "etc", "access.boostrap.json"), accessBootstrapPath)
		if err != nil {
			if os.IsNotExist(err) {
				return errorutils.CheckErrorf("%s: the file was not found or is not accessible", accessBootstrapPath)
			}
			return err
		}
	}
	return nil
}
