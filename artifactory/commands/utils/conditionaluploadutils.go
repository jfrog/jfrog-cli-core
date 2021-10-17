package utils

import (
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// ScanDeployableArtifacts scans all files founds in the given parsed deployableArtifacts results.
// If the scan passes, the method will return two filespec ready for upload, thee first one contains all the binaries
// and the seconde all the pom.xml's.
// If one of the file's scan failed both of the return values will be nil.
func ScanDeployableArtifacts(deployableArtifacts *Result, serverDetails *config.ServerDetails, threads int) (*spec.SpecFiles, *spec.SpecFiles, error) {
	binariesSpecFile := &spec.SpecFiles{}
	pomSpecFile := &spec.SpecFiles{}
	deployableArtifacts.Reader().Reset()
	for item := new(clientutils.FileTransferDetails); deployableArtifacts.Reader().NextRecord(item) == nil; item = new(clientutils.FileTransferDetails) {
		file := spec.File{Pattern: item.SourcePath, Target: parseTargetPath(item.TargetPath, serverDetails)}
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
	xrScanCmd := audit.NewScanCommand().SetServerDetails(serverDetails).SetSpec(binariesSpecFile).SetThreads(threads)
	err := xrScanCmd.Run()
	if err != nil {
		return nil, nil, err
	}
	if !xrScanCmd.IsScanPassed() {
		log.Info("Xray scan failed. No Artifact will be deployed")
		return nil, nil, nil
	}
	return binariesSpecFile, pomSpecFile, nil
}

func parseTargetPath(target string, serverDetails *config.ServerDetails) string {
	if strings.Contains(target, serverDetails.ArtifactoryUrl) {
		return target[len(serverDetails.ArtifactoryUrl):]
	}
	return target
}
