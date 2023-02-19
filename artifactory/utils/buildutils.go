package utils

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jfrog/build-info-go/build"
	buildInfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	artClientUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	BuildInfoDetails          = "details"
	BuildTempPath             = "jfrog/builds/"
	ProjectConfigBuildNameKey = "name"
)

func PrepareBuildPrerequisites(buildConfiguration *BuildConfiguration) (build *build.Build, err error) {
	// Prepare build-info.
	toCollect, err := buildConfiguration.IsCollectBuildInfo()
	if err != nil {
		return
	}
	if toCollect {
		log.Debug("Preparing build prerequisites...")
		var buildName, buildNumber string
		buildName, err = buildConfiguration.GetBuildName()
		if err != nil {
			return
		}
		buildNumber, err = buildConfiguration.GetBuildNumber()
		if err != nil {
			return
		}
		projectKey := buildConfiguration.GetProject()
		buildInfoService := CreateBuildInfoService()
		build, err = buildInfoService.GetOrCreateBuildWithProject(buildName, buildNumber, projectKey)
		if err != nil {
			err = errorutils.CheckError(err)
		}
	}

	return
}

func GetBuildDir(buildName, buildNumber, projectKey string) (string, error) {
	hash := sha256.Sum256([]byte(buildName + "_" + buildNumber + "_" + projectKey))
	buildsDir := filepath.Join(coreutils.GetCliPersistentTempDirPath(), BuildTempPath, hex.EncodeToString(hash[:]))
	err := os.MkdirAll(buildsDir, 0777)
	if errorutils.CheckError(err) != nil {
		return "", err
	}
	return buildsDir, nil
}

func CreateBuildProperties(buildName, buildNumber, projectKey string) (string, error) {
	if buildName == "" || buildNumber == "" {
		return "", nil
	}

	buildGeneralDetails, err := ReadBuildInfoGeneralDetails(buildName, buildNumber, projectKey)
	if err != nil {
		return fmt.Sprintf("build.name=%s;build.number=%s", buildName, buildNumber), err
	}
	timestamp := strconv.FormatInt(buildGeneralDetails.Timestamp.UnixNano()/int64(time.Millisecond), 10)
	return fmt.Sprintf("build.name=%s;build.number=%s;build.timestamp=%s", buildName, buildNumber, timestamp), nil
}

func getPartialsBuildDir(buildName, buildNumber, projectKey string) (string, error) {
	buildDir, err := GetBuildDir(buildName, buildNumber, projectKey)
	if err != nil {
		return "", err
	}
	buildDir = filepath.Join(buildDir, "partials")
	err = os.MkdirAll(buildDir, 0777)
	if errorutils.CheckError(err) != nil {
		return "", err
	}
	return buildDir, nil
}

func saveBuildData(action interface{}, buildName, buildNumber, projectKey string) (err error) {
	b, err := json.Marshal(&action)
	if errorutils.CheckError(err) != nil {
		return err
	}
	var content bytes.Buffer
	err = json.Indent(&content, b, "", "  ")
	if errorutils.CheckError(err) != nil {
		return err
	}
	dirPath, err := getPartialsBuildDir(buildName, buildNumber, projectKey)
	if err != nil {
		return err
	}
	log.Debug("Creating temp build file at:", dirPath)
	tempFile, err := os.CreateTemp(dirPath, "temp")
	if err != nil {
		return err
	}
	defer func() {
		e := tempFile.Close()
		if err == nil {
			err = errorutils.CheckError(e)
		}
	}()
	_, err = tempFile.Write(content.Bytes())
	return err
}

func SaveBuildInfo(buildName, buildNumber, projectKey string, buildInfo *buildInfo.BuildInfo) (err error) {
	b, err := json.Marshal(buildInfo)
	if errorutils.CheckError(err) != nil {
		return err
	}
	var content bytes.Buffer
	err = json.Indent(&content, b, "", "  ")
	if errorutils.CheckError(err) != nil {
		return err
	}
	dirPath, err := GetBuildDir(buildName, buildNumber, projectKey)
	if err != nil {
		return err
	}
	log.Debug("Creating temp build file at: " + dirPath)
	tempFile, err := os.CreateTemp(dirPath, "temp")
	if errorutils.CheckError(err) != nil {
		return err
	}
	defer func() {
		e := tempFile.Close()
		if err == nil {
			err = errorutils.CheckError(e)
		}
	}()
	_, err = tempFile.Write(content.Bytes())
	return errorutils.CheckError(err)
}

func SaveBuildGeneralDetails(buildName, buildNumber, projectKey string) error {
	partialsBuildDir, err := getPartialsBuildDir(buildName, buildNumber, projectKey)
	if err != nil {
		return err
	}
	log.Debug("Saving build general details at: " + partialsBuildDir)
	detailsFilePath := filepath.Join(partialsBuildDir, BuildInfoDetails)
	var exists bool
	exists, err = fileutils.IsFileExists(detailsFilePath, false)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	meta := buildInfo.General{
		Timestamp: time.Now(),
	}
	b, err := json.Marshal(&meta)
	if err != nil {
		return errorutils.CheckError(err)
	}
	var content bytes.Buffer
	err = json.Indent(&content, b, "", "  ")
	if err != nil {
		return errorutils.CheckError(err)
	}
	err = os.WriteFile(detailsFilePath, content.Bytes(), 0600)
	return errorutils.CheckError(err)
}

type populatePartialBuildInfo func(partial *buildInfo.Partial)

func SavePartialBuildInfo(buildName, buildNumber, projectKey string, populatePartialBuildInfoFunc populatePartialBuildInfo) error {
	partialBuildInfo := new(buildInfo.Partial)
	partialBuildInfo.Timestamp = time.Now().UnixNano() / int64(time.Millisecond)
	populatePartialBuildInfoFunc(partialBuildInfo)
	return saveBuildData(partialBuildInfo, buildName, buildNumber, projectKey)
}

func GetGeneratedBuildsInfo(buildName, buildNumber, projectKey string) ([]*buildInfo.BuildInfo, error) {
	buildDir, err := GetBuildDir(buildName, buildNumber, projectKey)
	if err != nil {
		return nil, err
	}
	buildFiles, err := fileutils.ListFiles(buildDir, false)
	if err != nil {
		return nil, err
	}

	var generatedBuildsInfo []*buildInfo.BuildInfo
	for _, buildFile := range buildFiles {
		dir, err := fileutils.IsDirExists(buildFile, false)
		if err != nil {
			return nil, err
		}
		if dir {
			continue
		}
		content, err := fileutils.ReadFile(buildFile)
		if err != nil {
			return nil, err
		}
		buildInfo := new(buildInfo.BuildInfo)
		err = json.Unmarshal(content, &buildInfo)
		if errorutils.CheckError(err) != nil {
			return nil, err
		}
		generatedBuildsInfo = append(generatedBuildsInfo, buildInfo)
	}
	return generatedBuildsInfo, nil
}

func ReadPartialBuildInfoFiles(buildName, buildNumber, projectKey string) (buildInfo.Partials, error) {
	var partials buildInfo.Partials
	partialsBuildDir, err := getPartialsBuildDir(buildName, buildNumber, projectKey)
	if err != nil {
		return nil, err
	}
	buildFiles, err := fileutils.ListFiles(partialsBuildDir, false)
	if err != nil {
		return nil, err
	}
	for _, buildFile := range buildFiles {
		dir, err := fileutils.IsDirExists(buildFile, false)
		if err != nil {
			return nil, err
		}
		if dir {
			continue
		}
		if strings.HasSuffix(buildFile, BuildInfoDetails) {
			continue
		}
		content, err := fileutils.ReadFile(buildFile)
		if err != nil {
			return nil, err
		}
		partial := new(buildInfo.Partial)
		err = json.Unmarshal(content, &partial)
		if errorutils.CheckError(err) != nil {
			return nil, err
		}
		partials = append(partials, partial)
	}

	return partials, nil
}

func ReadBuildInfoGeneralDetails(buildName, buildNumber, projectKey string) (*buildInfo.General, error) {
	partialsBuildDir, err := getPartialsBuildDir(buildName, buildNumber, projectKey)
	if err != nil {
		return nil, err
	}
	generalDetailsFilePath := filepath.Join(partialsBuildDir, BuildInfoDetails)
	fileExists, err := fileutils.IsFileExists(generalDetailsFilePath, false)
	if err != nil {
		return nil, err
	}
	if !fileExists {
		var buildString string
		if projectKey != "" {
			buildString = fmt.Sprintf("build-name: <%s>, build-number: <%s> and project: <%s>", buildName, buildNumber, projectKey)
		} else {
			buildString = fmt.Sprintf("build-name: <%s> and build-number: <%s>", buildName, buildNumber)
		}
		return nil, errors.New("Failed to construct the build-info to be published. " +
			"This may be because there were no previous commands, which collected build-info for " + buildString)
	}
	content, err := fileutils.ReadFile(generalDetailsFilePath)
	if err != nil {
		return nil, err
	}
	details := new(buildInfo.General)
	err = json.Unmarshal(content, &details)
	if errorutils.CheckError(err) != nil {
		return nil, err
	}
	return details, nil
}

func RemoveBuildDir(buildName, buildNumber, projectKey string) error {
	tempDirPath, err := GetBuildDir(buildName, buildNumber, projectKey)
	if err != nil {
		return err
	}
	exists, err := fileutils.IsDirExists(tempDirPath, false)
	if err != nil {
		return err
	}
	if exists {
		return errorutils.CheckError(fileutils.RemoveTempDir(tempDirPath))
	}
	return nil
}

type BuildConfiguration struct {
	buildName            string
	buildNumber          string
	module               string
	project              string
	loadedFromConfigFile bool
}

func NewBuildConfiguration(buildName, buildNumber, module, project string) *BuildConfiguration {
	return &BuildConfiguration{buildName: buildName, buildNumber: buildNumber, module: module, project: project}
}

func (bc *BuildConfiguration) SetBuildName(buildName string) *BuildConfiguration {
	bc.buildName = buildName
	return bc
}

func (bc *BuildConfiguration) SetBuildNumber(buildNumber string) *BuildConfiguration {
	bc.buildNumber = buildNumber
	return bc
}

func (bc *BuildConfiguration) SetProject(project string) *BuildConfiguration {
	bc.project = project
	return bc
}

func (bc *BuildConfiguration) SetModule(module string) *BuildConfiguration {
	bc.module = module
	return bc
}

func (bc *BuildConfiguration) GetBuildName() (string, error) {
	if bc.buildName != "" {
		return bc.buildName, nil
	}
	// Resolve from env var.
	if bc.buildName = os.Getenv(coreutils.BuildName); bc.buildName != "" {
		return bc.buildName, nil
	}
	// Resolve from config file in '.jfrog' folder.
	var err error
	if bc.buildName, err = bc.getBuildNameFromConfigFile(); bc.buildName != "" {
		bc.loadedFromConfigFile = true
	}
	return bc.buildName, err
}

func (bc *BuildConfiguration) getBuildNameFromConfigFile() (string, error) {
	confFilePath, exist, err := GetProjectConfFilePath(Build)
	if os.IsPermission(err) {
		log.Debug("The 'build-name' cannot be read from JFrog config due to permission denied.")
		return "", nil
	}
	if err != nil || !exist {
		return "", err
	}
	vConfig, err := ReadConfigFile(confFilePath, YAML)
	if err != nil || vConfig == nil {
		return "", err
	}
	return vConfig.GetString(ProjectConfigBuildNameKey), nil
}

func (bc *BuildConfiguration) GetBuildNumber() (string, error) {
	if bc.buildNumber != "" {
		return bc.buildNumber, nil
	}
	// Resolve from env var.
	if bc.buildNumber = os.Getenv(coreutils.BuildNumber); bc.buildNumber != "" {
		return bc.buildNumber, nil
	}
	// If build name was resolve from build.yaml file, use 'LATEST' as build number.
	buildName, err := bc.GetBuildName()
	if err != nil {
		return "", err
	}
	if buildName != "" && bc.loadedFromConfigFile {
		bc.buildNumber = artClientUtils.LatestBuildNumberKey
	}
	return bc.buildNumber, nil
}

func (bc *BuildConfiguration) GetProject() string {
	if bc.project != "" {
		return bc.project
	}
	// Resolve from env var.
	bc.project = os.Getenv(coreutils.Project)
	return bc.project
}

func (bc *BuildConfiguration) GetModule() string {
	return bc.module
}

// Validates:
// 1. If the build number exists, the build name also exists (and vice versa).
// 2. If the modules exists, the build name/number are also exist (and vice versa).
func (bc *BuildConfiguration) ValidateBuildAndModuleParams() error {
	buildName, err := bc.GetBuildName()
	if err != nil {
		return err
	}
	buildNumber, err := bc.GetBuildNumber()
	if err != nil {
		return err
	}
	module := bc.GetModule()
	if err := bc.ValidateBuildParams(); err != nil {
		return err
	}
	if module != "" && buildName == "" && buildNumber == "" {
		return errorutils.CheckErrorf("the build-name and build-number options are mandatory when the module option is provided.")

	}
	return nil
}

// Validates that if the build number exists, the build name also exists (and vice versa).
func (bc *BuildConfiguration) ValidateBuildParams() error {
	buildName, err := bc.GetBuildName()
	if err != nil {
		return err
	}
	buildNumber, err := bc.GetBuildNumber()
	if err != nil {
		return err
	}
	if (buildName == "" && buildNumber != "") || (buildName != "" && buildNumber == "") {
		return errorutils.CheckErrorf("the build-name and build-number options cannot be provided separately")
	}
	return nil
}

func (bc *BuildConfiguration) IsCollectBuildInfo() (bool, error) {
	if bc == nil {
		return false, nil
	}
	buildName, err := bc.GetBuildName()
	if err != nil {
		return false, err
	}
	buildNumber, err := bc.GetBuildNumber()
	if err != nil {
		return false, err
	}
	return buildNumber != "" && buildName != "", nil
}

func (bc *BuildConfiguration) IsLoadedFromConfigFile() bool {
	return bc.loadedFromConfigFile
}

func PopulateBuildArtifactsAsPartials(buildArtifacts []buildInfo.Artifact, buildConfiguration *BuildConfiguration, moduleType buildInfo.ModuleType) error {
	populateFunc := func(partial *buildInfo.Partial) {
		partial.Artifacts = buildArtifacts
		partial.ModuleId = buildConfiguration.GetModule()
		partial.ModuleType = moduleType
	}
	buildName, err := buildConfiguration.GetBuildName()
	if err != nil {
		return err
	}
	buildNumber, err := buildConfiguration.GetBuildNumber()
	if err != nil {
		return err
	}
	return SavePartialBuildInfo(buildName, buildNumber, buildConfiguration.GetProject(), populateFunc)
}

func CreateBuildPropsFromConfiguration(buildConfiguration *BuildConfiguration) (string, error) {
	buildName, err := buildConfiguration.GetBuildName()
	if err != nil {
		return "", err
	}
	buildNumber, err := buildConfiguration.GetBuildNumber()
	if err != nil {
		return "", err
	}
	err = SaveBuildGeneralDetails(buildName, buildNumber, buildConfiguration.GetProject())
	if err != nil {
		return "", err
	}
	return CreateBuildProperties(buildName, buildNumber, buildConfiguration.GetProject())
}
