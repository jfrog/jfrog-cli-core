package replication

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/c-bata/go-prompt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	// Strings for prompt questions
	SelectConfigKeyMsg = "Select the next configuration key" + ioutils.PressTabMsg

	// Template types
	TemplateType = "templateType"
	JobType      = "jobType"
	Create       = "create"
	Pull         = "pull"
	Push         = "push"

	// Common replication configuration JSON keys
	ServerId               = "serverId"
	CronExp                = "cronExp"
	RepoKey                = "repoKey"
	TargetRepoKey          = "targetRepoKey"
	EnableEventReplication = "enableEventReplication"
	Enabled                = "enabled"
	SyncDeletes            = "syncDeletes"
	SyncProperties         = "syncProperties"
	SyncStatistics         = "syncStatistics"
	// Deprecated
	PathPrefix               = "pathPrefix"
	IncludePathPrefixPattern = "includePathPrefixPattern"
	SocketTimeoutMillis      = "socketTimeoutMillis"
	DisableProxy             = "disableProxy"
)

type ReplicationTemplateCommand struct {
	path string
}

func NewReplicationTemplateCommand() *ReplicationTemplateCommand {
	return &ReplicationTemplateCommand{}
}

func (rtc *ReplicationTemplateCommand) SetTemplatePath(path string) *ReplicationTemplateCommand {
	rtc.path = path
	return rtc
}

func (rtc *ReplicationTemplateCommand) CommandName() string {
	return "rt_replication_template"
}

func (rtc *ReplicationTemplateCommand) ServerDetails() (*config.ServerDetails, error) {
	// Since it's a local command, usage won't be reported.
	return nil, nil
}

func getArtifactoryServerIds() []prompt.Suggest {
	suggest := make([]prompt.Suggest, 0)
	if configurations, _ := config.GetAllServersConfigs(); configurations != nil {
		for _, conf := range configurations {
			suggest = append(suggest, prompt.Suggest{Text: conf.ServerId})
		}
	}
	return suggest
}

func (rtc *ReplicationTemplateCommand) Run() (err error) {
	replicationTemplateQuestionnaire := &ioutils.InteractiveQuestionnaire{
		MandatoryQuestionsKeys: []string{JobType, RepoKey},
		QuestionsMap:           questionMap,
	}
	err = replicationTemplateQuestionnaire.Perform()
	if err != nil {
		return err
	}
	resBytes, err := json.Marshal(replicationTemplateQuestionnaire.AnswersMap)
	if err != nil {
		return errorutils.CheckError(err)
	}
	if err = os.WriteFile(rtc.path, resBytes, 0644); err != nil {
		return errorutils.CheckError(err)
	}
	log.Info(fmt.Sprintf("Replication creation config template successfully created at %s.", rtc.path))
	return nil
}

var questionMap = map[string]ioutils.QuestionInfo{
	ioutils.OptionalKey: {
		Msg:          "",
		PromptPrefix: "Select the next property >",
		AllowVars:    false,
		Writer:       nil,
		MapKey:       "",
		Callback:     ioutils.OptionalKeyCallback,
	},
	TemplateType: {
		Options: []prompt.Suggest{
			{Text: Create, Description: "Template for creating a new replication"},
		},
		Msg:          "",
		PromptPrefix: "Select the template type >",
		AllowVars:    false,
		Writer:       nil,
		MapKey:       "",
		Callback:     nil,
	},
	JobType: {
		Options: []prompt.Suggest{
			{Text: Pull, Description: "Pull replication"},
			{Text: Push, Description: "Push replication"},
		},
		Msg:          "",
		PromptPrefix: "Select replication job type" + ioutils.PressTabMsg,
		AllowVars:    false,
		Writer:       nil,
		MapKey:       "",
		Callback:     jobTypeCallback,
	},
	RepoKey: {
		Msg:          "",
		PromptPrefix: "Enter source repo key >",
		AllowVars:    true,
		Writer:       ioutils.WriteStringAnswer,
		MapKey:       RepoKey,
		Callback:     nil,
	},
	TargetRepoKey: {
		Msg:          "",
		PromptPrefix: "Enter target repo key >",
		AllowVars:    true,
		Writer:       ioutils.WriteStringAnswer,
		MapKey:       TargetRepoKey,
		Callback:     nil,
	},
	ServerId: {
		Options:      getArtifactoryServerIds(),
		Msg:          "",
		PromptPrefix: "Enter target server id" + ioutils.PressTabMsg,
		AllowVars:    true,
		Writer:       ioutils.WriteStringAnswer,
		MapKey:       ServerId,
		Callback:     nil,
	},
	CronExp: {
		Msg:          "",
		PromptPrefix: "Enter cron expression for frequency (for example, 0 0 12 * * ? will replicate daily) >",
		AllowVars:    true,
		Writer:       ioutils.WriteStringAnswer,
		MapKey:       CronExp,
		Callback:     nil,
	},
	EnableEventReplication: BoolToStringQuestionInfo,
	Enabled:                BoolToStringQuestionInfo,
	SyncDeletes:            BoolToStringQuestionInfo,
	SyncProperties:         BoolToStringQuestionInfo,
	SyncStatistics:         BoolToStringQuestionInfo,
	IncludePathPrefixPattern: {
		Msg:          "",
		PromptPrefix: "Enter include path prefix pattern >",
		AllowVars:    true,
		Writer:       ioutils.WriteStringAnswer,
		MapKey:       IncludePathPrefixPattern,
		Callback:     nil,
	},
	SocketTimeoutMillis: {
		Msg:          "",
		PromptPrefix: "Enter socket timeout millis >",
		AllowVars:    true,
		Writer:       ioutils.WriteStringAnswer,
		MapKey:       SocketTimeoutMillis,
		Callback:     nil,
	},
}

func jobTypeCallback(iq *ioutils.InteractiveQuestionnaire, jobType string) (string, error) {
	if jobType == Pull {
		iq.MandatoryQuestionsKeys = append(iq.MandatoryQuestionsKeys, CronExp)
	} else {
		iq.MandatoryQuestionsKeys = append(iq.MandatoryQuestionsKeys, TargetRepoKey, ServerId, CronExp)
	}
	iq.OptionalKeysSuggests = getAllPossibleOptionalRepoConfKeys()
	return "", nil
}

func getAllPossibleOptionalRepoConfKeys(values ...string) []prompt.Suggest {
	optionalKeys := []string{ioutils.SaveAndExit, Enabled, SyncDeletes, SyncProperties, SyncStatistics, PathPrefix, IncludePathPrefixPattern, EnableEventReplication, SocketTimeoutMillis}
	if len(values) > 0 {
		optionalKeys = append(optionalKeys, values...)
	}
	return ioutils.GetSuggestsFromKeys(optionalKeys, suggestionMap)
}

// Specific writers for repo templates, since all the values in the templates should be written as string.
var BoolToStringQuestionInfo = ioutils.QuestionInfo{
	Options:   ioutils.GetBoolSuggests(),
	AllowVars: true,
	Writer:    ioutils.WriteStringAnswer,
}

var suggestionMap = map[string]prompt.Suggest{
	ioutils.SaveAndExit:      {Text: ioutils.SaveAndExit},
	ServerId:                 {Text: ServerId},
	RepoKey:                  {Text: RepoKey},
	TargetRepoKey:            {Text: TargetRepoKey},
	CronExp:                  {Text: CronExp},
	EnableEventReplication:   {Text: EnableEventReplication},
	Enabled:                  {Text: Enabled},
	SyncDeletes:              {Text: SyncDeletes},
	SyncProperties:           {Text: SyncProperties},
	SyncStatistics:           {Text: SyncStatistics},
	PathPrefix:               {Text: PathPrefix},
	IncludePathPrefixPattern: {Text: IncludePathPrefixPattern},
	SocketTimeoutMillis:      {Text: SocketTimeoutMillis},
	DisableProxy:             {Text: DisableProxy},
}
