package replication

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/jfrog/jfrog-cli-core/artifactory/commands/utils"
	rtUtils "github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-cli-core/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
)

type ReplicationCreateCommand struct {
	rtDetails    *config.ArtifactoryDetails
	templatePath string
	vars         string
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

func (rcc *ReplicationCreateCommand) SetRtDetails(rtDetails *config.ArtifactoryDetails) *ReplicationCreateCommand {
	rcc.rtDetails = rtDetails
	return rcc
}

func (rcc *ReplicationCreateCommand) RtDetails() (*config.ArtifactoryDetails, error) {
	return rcc.rtDetails, nil
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
	// Go over the the confMap and write the values with the correct type using the writersMap
	serverId := ""
	for key, value := range replicationConfigMap {
		if err = utils.ValidateMapEntry(key, value, writersMap); err != nil {
			return
		}
		if key == "serverId" {
			serverId = value.(string)
		} else {
			writersMap[key](&replicationConfigMap, key, value.(string))
		}
	}
	fillMissingDefaultValue(replicationConfigMap)
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
	servicesManager, err := rtUtils.CreateServiceManager(rcc.rtDetails, false)
	if err != nil {
		return err
	}
	// In case 'serverId' is not found, pull replication will be assumed.
	if serverId != "" {
		if targetRepo, ok := replicationConfigMap["targetRepoKey"]; ok {
			if err = updateArtifactoryInfo(&params, serverId, targetRepo.(string)); err != nil {
				return err
			}
		} else {
			return errorutils.CheckError(errors.New("expected 'targetRepoKey' field in the json template file."))
		}
	}
	return servicesManager.CreateReplication(params)
}

func fillMissingDefaultValue(replicationConfigMap map[string]interface{}) {
	if _, ok := replicationConfigMap["socketTimeoutMillis"]; !ok {
		writersMap["socketTimeoutMillis"](&replicationConfigMap, "socketTimeoutMillis", "15000")
	}
	if _, ok := replicationConfigMap["syncProperties"]; !ok {
		writersMap["syncProperties"](&replicationConfigMap, "syncProperties", "true")
	}
}

func updateArtifactoryInfo(param *services.CreateReplicationParams, serverId, targetRepo string) error {
	singleConfig, err := config.GetArtifactorySpecificConfig(serverId, true, false)
	if err != nil {
		return err
	}
	param.Url, param.Password, param.Username = strings.TrimSuffix(singleConfig.GetUrl(), "/")+"/"+targetRepo, singleConfig.GetPassword(), singleConfig.GetUser()
	return nil
}

var writersMap = map[string]utils.AnswerWriter{
	ServerId:               utils.WriteStringAnswer,
	RepoKey:                utils.WriteStringAnswer,
	TargetRepoKey:          utils.WriteStringAnswer,
	CronExp:                utils.WriteStringAnswer,
	EnableEventReplication: utils.WriteBoolAnswer,
	Enabled:                utils.WriteBoolAnswer,
	SyncDeletes:            utils.WriteBoolAnswer,
	SyncProperties:         utils.WriteBoolAnswer,
	SyncStatistics:         utils.WriteBoolAnswer,
	PathPrefix:             utils.WriteStringAnswer,
	SocketTimeoutMillis:    utils.WriteIntAnswer,
}
