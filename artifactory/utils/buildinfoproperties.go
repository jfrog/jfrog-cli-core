package utils

import (
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"

	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/spf13/viper"
)

const (
	HttpProxy = "HTTP_PROXY"
)

type BuildConfigMapping map[ProjectType][]*map[string]string

var buildTypeConfigMapping = BuildConfigMapping{
	Maven:  {&commonConfigMapping, &mavenConfigMapping},
	Gradle: {&commonConfigMapping, &gradleConfigMapping},
}

type ConfigType string

const (
	YAML       ConfigType = "yaml"
	PROPERTIES ConfigType = "properties"
)

// For key/value binding
const BuildName = "build.name"
const BuildNumber = "build.number"
const BuildProject = "build.project"
const BuildTimestamp = "build.timestamp"
const GeneratedBuildInfo = "buildInfo.generated"
const DeployableArtifacts = "deployable.artifacts.map"
const InsecureTls = "insecureTls"

const ResolverPrefix = "resolver."
const DeployerPrefix = "deployer."

const Repo = "repo"
const SnapshotRepo = "snapshotRepo"
const ReleaseRepo = "releaseRepo"

const ServerId = "serverId"
const Url = "url"
const Username = "username"
const Password = "password"
const DeployArtifacts = "artifacts"

const MavenDescriptor = "deployMavenDescriptors"
const IvyDescriptor = "deployIvyDescriptors"
const IvyPattern = "ivyPattern"
const ArtifactPattern = "artifactPattern"
const ForkCount = "forkCount"

const IncludePatterns = "includePatterns"
const ExcludePatterns = "excludePatterns"
const FilterExcludedArtifactsFromBuild = "filterExcludedArtifactsFromBuild"

// For path and temp files
const PropertiesTempPrefix = "buildInfoProperties"
const PropertiesTempPath = "jfrog/properties/"
const GeneratedBuildInfoTempPrefix = "generatedBuildInfo"

const Proxy = "proxy."
const Host = "host"
const Port = "port"

// Config mapping are used to create buildInfo properties file to be used by BuildInfo extractors.
// Build config provided by the user may contain other properties that will not be included in the properties file.
var defaultPropertiesValues = map[string]string{
	"artifactory.publish.artifacts":                        "true",
	"artifactory.publish.buildInfo":                        "false",
	"artifactory.publish.unstable":                         "false",
	"artifactory.publish.maven":                            "false",
	"artifactory.publish.ivy":                              "false",
	"buildInfoConfig.includeEnvVars":                       "false",
	"buildInfoConfig.envVarsExcludePatterns":               "*password*,*psw*,*secret*,*key*,*token*",
	"buildInfo.agent.name":                                 coreutils.GetClientAgentName() + "/" + coreutils.GetClientAgentVersion(),
	"org.jfrog.build.extractor.maven.recorder.activate":    "true",
	"buildInfo.env.extractor.used":                         "true",
	"artifactory.publish.forkCount":                        "3",
	"artifactory.publish.filterExcludedArtifactsFromBuild": "true",
}

var commonConfigMapping = map[string]string{
	"artifactory.publish.buildInfo":          "",
	"artifactory.publish.unstable":           "",
	"buildInfoConfig.includeEnvVars":         "",
	"buildInfoConfig.envVarsExcludePatterns": "",
	"buildInfo.agent.name":                   "",
	"artifactory.resolve.contextUrl":         ResolverPrefix + Url,
	"artifactory.resolve.username":           ResolverPrefix + Username,
	"artifactory.resolve.password":           ResolverPrefix + Password,
	"artifactory.publish.contextUrl":         DeployerPrefix + Url,
	"artifactory.publish.username":           DeployerPrefix + Username,
	"artifactory.publish.password":           DeployerPrefix + Password,
	"artifactory.publish.artifacts":          DeployerPrefix + DeployArtifacts,
	"artifactory.deploy.build.name":          BuildName,
	"artifactory.deploy.build.number":        BuildNumber,
	"artifactory.deploy.build.project":       BuildProject,
	"artifactory.deploy.build.timestamp":     BuildTimestamp,
	"buildInfo.generated.build.info":         GeneratedBuildInfo,
	"buildInfo.deployable.artifacts.map":     DeployableArtifacts,
	"artifactory.proxy.host":                 Proxy + Host,
	"artifactory.proxy.port":                 Proxy + Port,
	"artifactory.publish.forkCount":          ForkCount,
	"artifactory.insecureTls":                InsecureTls,
}

var mavenConfigMapping = map[string]string{
	"org.jfrog.build.extractor.maven.recorder.activate":    "",
	"artifactory.resolve.repoKey":                          ResolverPrefix + ReleaseRepo,
	"artifactory.resolve.downSnapshotRepoKey":              ResolverPrefix + SnapshotRepo,
	"artifactory.publish.repoKey":                          DeployerPrefix + ReleaseRepo,
	"artifactory.publish.snapshot.repoKey":                 DeployerPrefix + SnapshotRepo,
	"artifactory.publish.includePatterns":                  DeployerPrefix + IncludePatterns,
	"artifactory.publish.excludePatterns":                  DeployerPrefix + ExcludePatterns,
	"artifactory.publish.filterExcludedArtifactsFromBuild": DeployerPrefix + FilterExcludedArtifactsFromBuild,
}

var gradleConfigMapping = map[string]string{
	"buildInfo.env.extractor.used":                      "",
	"org.jfrog.build.extractor.maven.recorder.activate": "",
	"artifactory.resolve.repoKey":                       ResolverPrefix + Repo,
	"artifactory.resolve.downSnapshotRepoKey":           ResolverPrefix + Repo,
	"artifactory.publish.repoKey":                       DeployerPrefix + Repo,
	"artifactory.publish.snapshot.repoKey":              DeployerPrefix + Repo,
	"artifactory.publish.maven":                         DeployerPrefix + MavenDescriptor,
	"artifactory.publish.ivy":                           DeployerPrefix + IvyDescriptor,
	"artifactory.publish.ivy.ivyPattern":                DeployerPrefix + IvyPattern,
	"artifactory.publish.ivy.artPattern":                DeployerPrefix + ArtifactPattern,
	"buildInfo.build.name":                              BuildName,
	"buildInfo.build.number":                            BuildNumber,
}

func ReadConfigFile(configPath string, configType ConfigType) (config *viper.Viper, err error) {
	config = viper.New()
	config.SetConfigType(string(configType))

	f, err := os.Open(configPath)
	if err != nil {
		return config, errorutils.CheckError(err)
	}
	defer func() {
		err = errorutils.CheckError(f.Close())
	}()
	err = config.ReadConfig(f)
	return config, errorutils.CheckError(err)
}

// Returns the Artifactory details
// Checks first for the deployer information if exists and if not, checks for the resolver information.
func GetServerDetails(vConfig *viper.Viper) (*config.ServerDetails, error) {
	if vConfig.IsSet(DeployerPrefix + ServerId) {
		serverId := vConfig.GetString(DeployerPrefix + ServerId)
		return config.GetSpecificConfig(serverId, true, true)
	}

	if vConfig.IsSet(ResolverPrefix + ServerId) {
		serverId := vConfig.GetString(ResolverPrefix + ServerId)
		return config.GetSpecificConfig(serverId, true, true)
	}
	return nil, nil
}

func CreateBuildInfoPropertiesFile(buildName, buildNumber, projectKey, deployableArtifactsFile string, config *viper.Viper, projectType ProjectType) (string, error) {
	if config.GetString("type") != projectType.String() {
		return "", errorutils.CheckErrorf("Incompatible build config, expected: " + projectType.String() + " got: " + config.GetString("type"))
	}

	propertiesPath := filepath.Join(coreutils.GetCliPersistentTempDirPath(), PropertiesTempPath)
	err := os.MkdirAll(propertiesPath, 0777)
	if errorutils.CheckError(err) != nil {
		return "", err
	}
	propertiesFile, err := ioutil.TempFile(propertiesPath, PropertiesTempPrefix)
	defer propertiesFile.Close()
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	err = setServerDetailsToConfig(ResolverPrefix, config)
	if err != nil {
		return "", err
	}
	err = setServerDetailsToConfig(DeployerPrefix, config)
	if err != nil {
		return "", err
	}
	if buildName != "" || buildNumber != "" {
		err = SaveBuildGeneralDetails(buildName, buildNumber, projectKey)
		if err != nil {
			return "", err
		}
		err = setBuildTimestampToConfig(buildName, buildNumber, projectKey, config)
		if err != nil {
			return "", err
		}
		err = createGeneratedBuildInfoFile(buildName, buildNumber, projectKey, config)
		if err != nil {
			return "", err
		}
	}
	err = setProxyIfDefined(config)
	if err != nil {
		return "", err
	}
	if deployableArtifactsFile != "" {
		config.Set(DeployableArtifacts, deployableArtifactsFile)
	}

	// Iterate over all the required properties keys according to the buildType and create properties file.
	// If a value is provided by the build config file write it,
	// otherwise use the default value from defaultPropertiesValues map.
	for _, partialMapping := range buildTypeConfigMapping[projectType] {
		for propertyKey, configKey := range *partialMapping {
			if config.IsSet(configKey) {
				_, err = propertiesFile.WriteString(propertyKey + "=" + config.GetString(configKey) + "\n")
			} else if defaultVal, ok := defaultPropertiesValues[propertyKey]; ok {
				_, err = propertiesFile.WriteString(propertyKey + "=" + defaultVal + "\n")
			}
			if err != nil {
				return "", errorutils.CheckError(err)
			}
		}
	}
	return propertiesFile.Name(), nil
}

// If the HTTP_PROXY environment variable is set, add to the config proxy details.
func setProxyIfDefined(config *viper.Viper) error {
	// Add HTTP_PROXY if exists
	proxy := os.Getenv(HttpProxy)
	if proxy != "" {
		url, err := url.Parse(proxy)
		if err != nil {
			return errorutils.CheckError(err)
		}
		host, port, err := net.SplitHostPort(url.Host)
		if err != nil {
			return errorutils.CheckError(err)
		}
		config.Set(Proxy+Host, host)
		config.Set(Proxy+Port, port)
	}
	return nil
}

func setServerDetailsToConfig(contextPrefix string, vConfig *viper.Viper) error {
	if !vConfig.IsSet(contextPrefix + ServerId) {
		return nil
	}

	serverId := vConfig.GetString(contextPrefix + ServerId)
	artDetails, err := config.GetSpecificConfig(serverId, true, true)
	if err != nil {
		return err
	}
	if artDetails.GetArtifactoryUrl() == "" {
		return errorutils.CheckErrorf("Server ID " + serverId + ": URL is required.")
	}
	vConfig.Set(contextPrefix+Url, artDetails.GetArtifactoryUrl())

	if artDetails.GetAccessToken() != "" {
		username, err := auth.ExtractUsernameFromAccessToken(artDetails.GetAccessToken())
		if err != nil {
			return err
		}
		vConfig.Set(contextPrefix+Username, username)
		vConfig.Set(contextPrefix+Password, artDetails.GetAccessToken())
		return nil
	}

	if artDetails.GetUser() != "" && artDetails.GetPassword() != "" {
		vConfig.Set(contextPrefix+Username, artDetails.GetUser())
		vConfig.Set(contextPrefix+Password, artDetails.GetPassword())
	}
	return nil
}

// Generated build info file is template file where build-info will be written to during the
// Maven or Gradle build.
// Creating this file only if build name and number is provided.
func createGeneratedBuildInfoFile(buildName, buildNumber, projectKey string, config *viper.Viper) error {
	config.Set(BuildName, buildName)
	config.Set(BuildNumber, buildNumber)
	config.Set(BuildProject, projectKey)

	buildPath, err := GetBuildDir(buildName, buildNumber, projectKey)
	if err != nil {
		return err
	}
	var tempFile *os.File
	tempFile, err = ioutil.TempFile(buildPath, GeneratedBuildInfoTempPrefix)
	defer tempFile.Close()
	if err != nil {
		return err
	}
	// If this is a Windows machine there is a need to modify the path for the build info file to match Java syntax with double \\
	path := ioutils.DoubleWinPathSeparator(tempFile.Name())
	config.Set(GeneratedBuildInfo, path)
	return nil
}

func setBuildTimestampToConfig(buildName, buildNumber, projectKey string, config *viper.Viper) error {
	buildGeneralDetails, err := ReadBuildInfoGeneralDetails(buildName, buildNumber, projectKey)
	if err != nil {
		return err
	}
	config.Set(BuildTimestamp, strconv.FormatInt(buildGeneralDetails.Timestamp.UnixNano()/int64(time.Millisecond), 10))
	return nil
}
