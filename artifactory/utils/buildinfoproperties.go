package utils

import (
	"net"
	"net/url"
	"os"
	"strings"

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
	"publish.artifacts":                                 "true",
	"publish.buildInfo":                                 "false",
	"publish.unstable":                                  "false",
	"publish.maven":                                     "false",
	"publish.ivy":                                       "false",
	"buildInfoConfig.includeEnvVars":                    "false",
	"buildInfoConfig.envVarsExcludePatterns":            "*password*,*psw*,*secret*,*key*,*token*,*auth*",
	"buildInfo.agent.name":                              coreutils.GetClientAgentName() + "/" + coreutils.GetClientAgentVersion(),
	"org.jfrog.build.extractor.maven.recorder.activate": "true",
	"buildInfo.env.extractor.used":                      "true",
	"publish.forkCount":                                 "3",
	"publish.filterExcludedArtifactsFromBuild":          "true",
}

var commonConfigMapping = map[string]string{
	"publish.buildInfo":                      "",
	"publish.unstable":                       "",
	"buildInfoConfig.includeEnvVars":         "",
	"buildInfoConfig.envVarsExcludePatterns": "",
	"buildInfo.agent.name":                   "",
	"resolve.contextUrl":                     ResolverPrefix + Url,
	"resolve.username":                       ResolverPrefix + Username,
	"resolve.password":                       ResolverPrefix + Password,
	"publish.contextUrl":                     DeployerPrefix + Url,
	"publish.username":                       DeployerPrefix + Username,
	"publish.password":                       DeployerPrefix + Password,
	"publish.artifacts":                      DeployerPrefix + DeployArtifacts,
	"deploy.build.name":                      BuildName,
	"deploy.build.number":                    BuildNumber,
	"deploy.build.project":                   BuildProject,
	"deploy.build.timestamp":                 BuildTimestamp,
	"buildInfo.generated.build.info":         GeneratedBuildInfo,
	"buildInfo.deployable.artifacts.map":     DeployableArtifacts,
	"proxy.host":                             Proxy + Host,
	"proxy.port":                             Proxy + Port,
	"publish.forkCount":                      ForkCount,
	"insecureTls":                            InsecureTls,
}

var mavenConfigMapping = map[string]string{
	"org.jfrog.build.extractor.maven.recorder.activate": "",
	"buildInfoConfig.artifactoryResolutionEnabled":      "buildInfoConfig.artifactoryResolutionEnabled",
	"resolve.repoKey":                          ResolverPrefix + ReleaseRepo,
	"resolve.downSnapshotRepoKey":              ResolverPrefix + SnapshotRepo,
	"publish.repoKey":                          DeployerPrefix + ReleaseRepo,
	"publish.snapshot.repoKey":                 DeployerPrefix + SnapshotRepo,
	"publish.includePatterns":                  DeployerPrefix + IncludePatterns,
	"publish.excludePatterns":                  DeployerPrefix + ExcludePatterns,
	"publish.filterExcludedArtifactsFromBuild": DeployerPrefix + FilterExcludedArtifactsFromBuild,
}

var gradleConfigMapping = map[string]string{
	"buildInfo.env.extractor.used":                      "",
	"org.jfrog.build.extractor.maven.recorder.activate": "",
	"resolve.repoKey":                                   ResolverPrefix + Repo,
	"resolve.downSnapshotRepoKey":                       ResolverPrefix + Repo,
	"publish.repoKey":                                   DeployerPrefix + Repo,
	"publish.snapshot.repoKey":                          DeployerPrefix + Repo,
	"publish.maven":                                     DeployerPrefix + MavenDescriptor,
	"publish.ivy":                                       DeployerPrefix + IvyDescriptor,
	"publish.ivy.ivyPattern":                            DeployerPrefix + IvyPattern,
	"publish.ivy.artPattern":                            DeployerPrefix + ArtifactPattern,
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

func ReadGradleConfig(path string, useWrapperIfMissingConfig bool) (config *viper.Viper, err error) {
	if path == "" {
		config = createDefaultGradleConfig(useWrapperIfMissingConfig)
	} else {
		config, err = ReadConfigFile(path, YAML)
	}
	return
}

func ReadMavenConfig(path string) (config *viper.Viper, err error) {
	if path == "" {
		config = createDefaultMavenConfig()
	} else {
		config, err = ReadConfigFile(path, YAML)
	}
	return
}

func createDefaultMavenConfig() *viper.Viper {
	vConfig := viper.New()
	vConfig.SetConfigType(string(YAML))
	vConfig.Set("type", Maven.String())
	return vConfig
}

func createDefaultGradleConfig(useWrapperIfMissingConfig bool) *viper.Viper {
	vConfig := viper.New()
	vConfig.SetConfigType(string(YAML))
	vConfig.Set("type", Gradle.String())
	vConfig.Set("usewrapper", useWrapperIfMissingConfig)
	return vConfig
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

func CreateBuildInfoProps(buildArtifactsDetailsFile string, config *viper.Viper, projectType ProjectType) (map[string]string, error) {
	if config.GetString("type") != projectType.String() {
		return nil, errorutils.CheckErrorf("Incompatible build config, expected: " + projectType.String() + " got: " + config.GetString("type"))
	}
	if err := setServerDetailsToConfig(ResolverPrefix, config); err != nil {
		return nil, err
	}
	if err := setServerDetailsToConfig(DeployerPrefix, config); err != nil {
		return nil, err
	}
	if err := setProxyIfDefined(config); err != nil {
		return nil, err
	}
	if buildArtifactsDetailsFile != "" {
		config.Set(DeployableArtifacts, buildArtifactsDetailsFile)
	}
	return createProps(config, projectType), nil
}

func createProps(config *viper.Viper, projectType ProjectType) map[string]string {
	props := make(map[string]string)
	// Iterate over all the required properties keys according to the buildType and create properties file.
	// If a value is provided by the build config file write it,
	// otherwise use the default value from defaultPropertiesValues map.
	for _, partialMapping := range buildTypeConfigMapping[projectType] {
		for propKey, configKey := range *partialMapping {
			var value string
			if config.IsSet(configKey) {
				value = config.GetString(configKey)
			} else if defaultVal, ok := defaultPropertiesValues[propKey]; ok {
				value = defaultVal
			}
			if value != "" {
				props[propKey] = value
				// Properties that have the 'artifactory.' prefix are deprecated.
				// For backward compatibility reasons, both will be added to the props map.
				if !strings.HasPrefix(propKey, "artifactory.") {
					props["artifactory."+propKey] = value
				}
			}
		}
	}
	return props
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
