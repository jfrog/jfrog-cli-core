package generic

import (
	commandsutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

type GenericCommand struct {
	serverDetails          *config.ServerDetails
	spec                   *spec.SpecFiles
	result                 *commandsutils.Result
	dryRun                 bool
	detailedSummary        bool
	syncDeletesPath        string
	quiet                  bool
	retries                int
	retryWaitTimeMilliSecs int
	aqlInclude             []string
}

func NewGenericCommand() *GenericCommand {
	return &GenericCommand{result: new(commandsutils.Result)}
}

func (gc *GenericCommand) DryRun() bool {
	return gc.dryRun
}

func (gc *GenericCommand) SetDryRun(dryRun bool) *GenericCommand {
	gc.dryRun = dryRun
	return gc
}

func (gc *GenericCommand) SyncDeletesPath() string {
	return gc.syncDeletesPath
}

func (gc *GenericCommand) SetSyncDeletesPath(syncDeletes string) *GenericCommand {
	gc.syncDeletesPath = syncDeletes
	return gc
}

func (gc *GenericCommand) Quiet() bool {
	return gc.quiet
}

func (gc *GenericCommand) SetQuiet(quiet bool) *GenericCommand {
	gc.quiet = quiet
	return gc
}

func (gc *GenericCommand) Retries() int {
	return gc.retries
}

func (gc *GenericCommand) SetRetries(retries int) *GenericCommand {
	gc.retries = retries
	return gc
}

func (gc *GenericCommand) SetRetryWaitMilliSecs(retryWaitMilliSecs int) *GenericCommand {
	gc.retryWaitTimeMilliSecs = retryWaitMilliSecs
	return gc
}

func (gc *GenericCommand) Result() *commandsutils.Result {
	return gc.result
}

func (gc *GenericCommand) Spec() *spec.SpecFiles {
	return gc.spec
}

func (gc *GenericCommand) SetSpec(spec *spec.SpecFiles) *GenericCommand {
	gc.spec = spec
	return gc
}

func (gc *GenericCommand) ServerDetails() (*config.ServerDetails, error) {
	return gc.serverDetails, nil
}

func (gc *GenericCommand) SetServerDetails(serverDetails *config.ServerDetails) *GenericCommand {
	gc.serverDetails = serverDetails
	return gc
}

func (gc *GenericCommand) DetailedSummary() bool {
	return gc.detailedSummary
}

func (gc *GenericCommand) SetDetailedSummary(detailedSummary bool) *GenericCommand {
	gc.detailedSummary = detailedSummary
	return gc
}

func (gc *GenericCommand) AqlInclue() []string {
	return gc.aqlInclude
}

func (gc *GenericCommand) SetAqlInclude(include []string) *GenericCommand {
	gc.aqlInclude = include
	return gc
}
