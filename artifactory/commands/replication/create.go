package replication

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	rtUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
)

type ReplicationCreateCommand struct {
	serverDetails *config.ServerDetails
	templatePath  string
	vars          string
}

func NewReplicationCreateCommand() *ReplicationCreateCommand {
	return &ReplicationCreateCommand{}
}

func (rcc *ReplicationCreateCommand) SetTemplatePath(path string) *ReplicationCreateCommand {
	rcc.templatePath = path
	return rcc
}

func (rcc *ReplicationCreateCommand) SetVars(vars string) *ReplicationCreateCommand {
	rcc.vars = vars
	return rcc
}

func (rcc *ReplicationCreateCommand) SetServerDetails(serverDetails *config.ServerDetails) *ReplicationCreateCommand {
	rcc.serverDetails = serverDetails
	return rcc
}

func (rcc *ReplicationCreateCommand) ServerDetails() (*config.ServerDetails, error) {
	return rcc.serverDetails, nil
}

func (rcc *ReplicationCreateCommand) CommandName() string {
	return "rt_replication_create"
}

func (rcc *ReplicationCreateCommand) Run() (err error) {
	content, err := fileutils.ReadFile(rcc.templatePath)
	if errorutils.CheckError(err) != nil {
		return
	}
	// Replace vars string-by-string if needed
	if len(rcc.vars) > 0 {
		templateVars := coreutils.SpecVarsStringToMap(rcc.vars)
		content = coreutils.ReplaceVars(content, templateVars)
	}
	// Unmarshal template to a map
	var replicationConfigMap map[string]interface{}
	err = json.Unmarshal(content, &replicationConfigMap)
	if errorutils.CheckError(err) != nil {
		return
	}
	// All the values in the template are strings
	// Go over the confMap and write the values with the correct type using the writersMap
	serverId := ""
	for key, value := range replicationConfigMap {
		if err = utils.ValidateMapEntry(key, value, writersMap); err != nil {
			return
		}
		if key == "serverId" {
			serverId = fmt.Sprint(value)
		} else {
			err := writersMap[key](&replicationConfigMap, key, fmt.Sprint(value))
			if err != nil {
				return err
			}
		}
	}
	err = fillMissingDefaultValue(replicationConfigMap)
	if err != nil {
		return err
	}
	// Write a JSON with the correct values
	content, err = json.Marshal(replicationConfigMap)
	if errorutils.CheckError(err) != nil {
		return
	}
	var params services.CreateReplicationParams
	err = json.Unmarshal(content, &params)
	if errorutils.CheckError(err) != nil {
		return
	}

	setPathPrefixBackwardCompatibility(&params)
	servicesManager, err := rtUtils.CreateServiceManager(rcc.serverDetails, -1, 0, false)
	if err != nil {
		return err
	}
	// In case 'serverId' is not found, pull replication will be assumed.
	if serverId != "" {
		if targetRepo, ok := replicationConfigMap["targetRepoKey"]; ok {
			if err = updateArtifactoryInfo(&params, serverId, fmt.Sprint(targetRepo)); err != nil {
				return err
			}
		} else {
			return errorutils.CheckErrorf("expected 'targetRepoKey' field in the json template file.")
		}
	}
	return servicesManager.CreateReplication(params)
}

func fillMissingDefaultValue(replicationConfigMap map[string]interface{}) error {
	if _, ok := replicationConfigMap["socketTimeoutMillis"]; !ok {
		err := writersMap["socketTimeoutMillis"](&replicationConfigMap, "socketTimeoutMillis", "15000")
		if err != nil {
			return err
		}
	}
	if _, ok := replicationConfigMap["syncProperties"]; !ok {
		err := writersMap["syncProperties"](&replicationConfigMap, "syncProperties", "true")
		if err != nil {
			return err
		}
	}
	return nil
}

// Make the pathPrefix parameter equals to the includePathPrefixPattern to support Artifactory < 7.27.4
func setPathPrefixBackwardCompatibility(params *services.CreateReplicationParams) {
	if params.IncludePathPrefixPattern == "" {
		params.IncludePathPrefixPattern = params.PathPrefix
		return
	}
	if params.PathPrefix == "" {
		params.PathPrefix = params.IncludePathPrefixPattern
	}
}

func updateArtifactoryInfo(param *services.CreateReplicationParams, serverId, targetRepo string) error {
	singleConfig, err := config.GetSpecificConfig(serverId, true, false)
	if err != nil {
		return err
	}
	param.Url, param.Password, param.Username = strings.TrimSuffix(singleConfig.GetArtifactoryUrl(), "/")+"/"+targetRepo, singleConfig.GetPassword(), singleConfig.GetUser()
	return nil
}

var writersMap = map[string]utils.AnswerWriter{
	ServerId:                 utils.WriteStringAnswer,
	RepoKey:                  utils.WriteStringAnswer,
	TargetRepoKey:            utils.WriteStringAnswer,
	CronExp:                  utils.WriteStringAnswer,
	EnableEventReplication:   utils.WriteBoolAnswer,
	Enabled:                  utils.WriteBoolAnswer,
	SyncDeletes:              utils.WriteBoolAnswer,
	SyncProperties:           utils.WriteBoolAnswer,
	SyncStatistics:           utils.WriteBoolAnswer,
	PathPrefix:               utils.WriteStringAnswer,
	IncludePathPrefixPattern: utils.WriteStringAnswer,
	SocketTimeoutMillis:      utils.WriteIntAnswer,
	DisableProxy:             utils.WriteBoolAnswer,
}
