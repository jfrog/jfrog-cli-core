package utils

import (
	xraycommands "github.com/jfrog/jfrog-cli-core/v2/xray/commands/scan"
	xrutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// ScanDeployableArtifacts scans all files founds in the given parsed deployableArtifacts results.
// If the scan passes, the method returns two filespec ready for upload, the first one contains all the binaries
// and the seconde all the pom.xml's.
// If one of the file's scan failed both of the return values will be nil.
func ScanDeployableArtifacts(deployableArtifacts *Result, serverDetails *config.ServerDetails, threads int, format xrutils.OutputFormat) (*spec.SpecFiles, *spec.SpecFiles, error) {
	binariesSpecFile := &spec.SpecFiles{}
	pomSpecFile := &spec.SpecFiles{}
	deployableArtifacts.Reader().Reset()
	for item := new(clientutils.FileTransferDetails); deployableArtifacts.Reader().NextRecord(item) == nil; item = new(clientutils.FileTransferDetails) {
		file := spec.File{Pattern: item.SourcePath, Target: parseTargetPath(item.TargetPath, serverDetails.ArtifactoryUrl)}
		if strings.HasSuffix(item.SourcePath, "pom.xml") {
			pomSpecFile.Files = append(pomSpecFile.Files, file)
		} else {
			binariesSpecFile.Files = append(binariesSpecFile.Files, file)
		}
	}
	if err := deployableArtifacts.Reader().GetError(); err != nil {
		return nil, nil, err
	}
	// Only non pom.xml should be scanned
	xrScanCmd := xraycommands.NewScanCommand().SetServerDetails(serverDetails).SetSpec(binariesSpecFile).SetThreads(threads).SetOutputFormat(format)
	err := xrScanCmd.Run()
	if err != nil {
		return nil, nil, err
	}
	if !xrScanCmd.IsScanPassed() {
		log.Info("Violations were found by Xray. No artifacts will be deployed")
		return nil, nil, nil
	}
	return binariesSpecFile, pomSpecFile, nil
}

// Returns the target path inside a given server URL.
func parseTargetPath(target, serverUrl string) string {
	if strings.Contains(target, serverUrl) {
		return target[len(serverUrl):]
	}
	return target
}

func GetXrayOutputFormat(formatFlagVal string) (format xrutils.OutputFormat, err error) {
	// Default print format is table.
	format = xrutils.Table
	if formatFlagVal != "" {
		switch strings.ToLower(formatFlagVal) {
		case string(xrutils.Table):
			format = xrutils.Table
		case string(xrutils.Json):
			format = xrutils.Json
		default:
			err = errorutils.CheckErrorf("only the following output formats are supported: table or json")
		}
	}
	return
}
