package golang

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/gofrog/crypto"
	"github.com/jfrog/gofrog/version"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-client-go/artifactory"
	_go "github.com/jfrog/jfrog-client-go/artifactory/services/go"
	servicesutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// Publish go project to Artifactory.
func publishPackage(packageVersion, targetRepo, buildName, buildNumber, projectKey string, excludedPatterns []string, servicesManager artifactory.ArtifactoryServicesManager) (summary *servicesutils.OperationSummary, artifacts []buildinfo.Artifact, err error) {
	projectPath, err := getProjectRoot()
	if err != nil {
		return nil, nil, errorutils.CheckError(err)
	}

	// Read module name
	moduleName, err := GetModuleName(projectPath)
	if err != nil {
		return nil, nil, err
	}

	log.Info("Publishing", moduleName, "to", targetRepo)
	filePathInRepo := path.Join(moduleName, "@v", packageVersion)
	collectBuildInfo := len(buildName) > 0 && len(buildNumber) > 0
	modContent, modArtifact, err := readModFile(packageVersion, projectPath, targetRepo, filePathInRepo+".mod", collectBuildInfo)
	if err != nil {
		return nil, nil, err
	}

	props, err := build.CreateBuildProperties(buildName, buildNumber, projectKey)
	if err != nil {
		return nil, nil, err
	}

	// Temp directory for the project archive.
	// The directory will be deleted at the end.
	tempDirPath, err := fileutils.CreateTempDir()
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		err = errors.Join(err, fileutils.RemoveTempDir(tempDirPath))
	}()

	var zipArtifact *buildinfo.Artifact
	params := _go.NewGoParams()
	params.Version = packageVersion
	params.Props = props
	params.TargetRepo = targetRepo
	params.ModuleId = moduleName
	params.ModContent = modContent
	params.ModPath = filepath.Join(projectPath, "go.mod")
	params.ZipPath, zipArtifact, err = archive(moduleName, packageVersion, projectPath, targetRepo, tempDirPath, excludedPatterns)
	if err != nil {
		return nil, nil, err
	}
	if collectBuildInfo {
		artifacts = []buildinfo.Artifact{*modArtifact, *zipArtifact}
	}

	// Create the info file if Artifactory version is 6.10.0 and above.
	artifactoryVersion, err := servicesManager.GetConfig().GetServiceDetails().GetVersion()
	if err != nil {
		return nil, nil, err
	}
	version := version.NewVersion(artifactoryVersion)
	if version.AtLeast(_go.ArtifactoryMinSupportedVersion) {
		log.Debug("Creating info file", projectPath)
		var pathToInfo string
		pathToInfo, err = createInfoFile(packageVersion)
		if err != nil {
			return nil, nil, err
		}
		defer func() {
			err = errors.Join(err, errorutils.CheckError(os.Remove(pathToInfo)))
		}()
		if collectBuildInfo {
			var infoArtifact *buildinfo.Artifact
			infoArtifact, err = createInfoFileArtifact(pathToInfo, packageVersion, targetRepo, filePathInRepo+".info")
			if err != nil {
				return nil, nil, err
			}
			artifacts = append(artifacts, *infoArtifact)
		}
		params.InfoPath = pathToInfo
	}

	summary, err = servicesManager.PublishGoProject(params)
	return summary, artifacts, err
}

// Creates the info file.
// Returns the path to that file.
func createInfoFile(packageVersion string) (path string, err error) {
	currentTime := time.Now().Format("2006-01-02T15:04:05Z")
	goInfoContent := goInfo{Version: packageVersion, Time: currentTime}
	content, err := json.Marshal(&goInfoContent)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	file, err := os.Create(packageVersion + ".info")
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	defer func() {
		err = errors.Join(err, errorutils.CheckError(file.Close()))
	}()
	_, err = file.Write(content)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	path, err = filepath.Abs(file.Name())
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	log.Debug("Info file was successfully created:", path)
	return path, nil
}

// Read go.mod file.
// Pass createArtifact = true to create an Artifact for build-info.
func readModFile(version, projectPath, deploymentRepo, relPathInRepo string, createArtifact bool) ([]byte, *buildinfo.Artifact, error) {
	modFilePath := filepath.Join(projectPath, "go.mod")
	modFileExists, _ := fileutils.IsFileExists(modFilePath, true)
	if !modFileExists {
		return nil, nil, errorutils.CheckErrorf("Could not find project's go.mod in " + projectPath)
	}
	modFile, err := os.Open(modFilePath)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		err = errors.Join(err, errorutils.CheckError(modFile.Close()))
	}()
	content, err := io.ReadAll(modFile)
	if err != nil {
		return nil, nil, errorutils.CheckError(err)
	}

	if !createArtifact {
		return content, nil, nil
	}

	checksums, err := crypto.CalcChecksums(bytes.NewBuffer(content))
	if err != nil {
		return nil, nil, errorutils.CheckError(err)
	}

	// Add mod file as artifact
	artifact := &buildinfo.Artifact{Name: version + ".mod", Type: "mod", OriginalDeploymentRepo: deploymentRepo, Path: relPathInRepo}
	artifact.Checksum = buildinfo.Checksum{Sha1: checksums[crypto.SHA1], Md5: checksums[crypto.MD5]}
	return content, artifact, nil
}

// Archive the go project.
// Returns the path of the temp archived project file.
func archive(moduleName, version, projectPath, deploymentRepo, tempDir string, excludedPatterns []string) (name string, zipArtifact *buildinfo.Artifact, err error) {
	openedFile := false
	tempFile, err := os.CreateTemp(tempDir, "project.zip")
	if err != nil {
		return "", nil, errorutils.CheckError(err)
	}
	openedFile = true
	defer func() {
		if openedFile {
			err = errors.Join(err, errorutils.CheckError(tempFile.Close()))
		}
	}()
	if err = archiveProject(tempFile, projectPath, moduleName, version, excludedPatterns); err != nil {
		return "", nil, errorutils.CheckError(err)
	}
	// Double-check that the paths within the zip file are well-formed.
	fi, err := tempFile.Stat()
	if err != nil {
		return "", nil, err
	}
	z, err := zip.NewReader(tempFile, fi.Size())
	if err != nil {
		return "", nil, err
	}
	prefix := moduleName + "@" + version + "/"
	for _, f := range z.File {
		if !strings.HasPrefix(f.Name, prefix) {
			return "", nil, fmt.Errorf("zip for %s has unexpected file %s", prefix[:len(prefix)-1], f.Name)
		}
	}
	// Sync the file before renaming it
	if err = tempFile.Sync(); err != nil {
		return "", nil, err
	}
	if err = tempFile.Close(); err != nil {
		return "", nil, err
	}
	openedFile = false
	fileDetails, err := fileutils.GetFileDetails(tempFile.Name(), true)
	if err != nil {
		return "", nil, err
	}

	zipArtifact = &buildinfo.Artifact{Name: version + ".zip", Type: "zip", OriginalDeploymentRepo: deploymentRepo, Path: path.Join(moduleName, "@v", version+".zip")}
	zipArtifact.Checksum = buildinfo.Checksum{Sha1: fileDetails.Checksum.Sha1, Md5: fileDetails.Checksum.Md5}
	return tempFile.Name(), zipArtifact, nil
}

// Add the info file also as an artifact to be part of the build info.
func createInfoFileArtifact(infoFilePath, packageVersion, targetRepo, relPathInRepo string) (*buildinfo.Artifact, error) {
	fileDetails, err := fileutils.GetFileDetails(infoFilePath, true)
	if err != nil {
		return nil, err
	}

	artifact := &buildinfo.Artifact{Name: packageVersion + ".info", Type: "info", OriginalDeploymentRepo: targetRepo, Path: relPathInRepo}
	artifact.Checksum = buildinfo.Checksum{Sha1: fileDetails.Checksum.Sha1, Md5: fileDetails.Checksum.Md5}
	return artifact, nil
}

type goInfo struct {
	Version string `json:"Version"`
	Time    string `json:"Time"`
}

func getProjectRoot() (string, error) {
	path, err := utils.GetProjectRoot()
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return path, nil
}
