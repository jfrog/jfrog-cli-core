package permissiontarget

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

type PermissionTargetUpdateCommand struct {
	PermissionTargetCommand
}

func NewPermissionTargetUpdateCommand() *PermissionTargetUpdateCommand {
	return &PermissionTargetUpdateCommand{}
}

func (ptuc *PermissionTargetUpdateCommand) SetTemplatePath(path string) *PermissionTargetUpdateCommand {
	ptuc.templatePath = path
	return ptuc
}

func (ptuc *PermissionTargetUpdateCommand) SetVars(vars string) *PermissionTargetUpdateCommand {
	ptuc.vars = vars
	return ptuc
}

func (ptuc *PermissionTargetUpdateCommand) SetServerDetails(serverDetails *config.ServerDetails) *PermissionTargetUpdateCommand {
	ptuc.serverDetails = serverDetails
	return ptuc
}

func (ptuc *PermissionTargetUpdateCommand) ServerDetails() (*config.ServerDetails, error) {
	return ptuc.serverDetails, nil
}

func (ptuc *PermissionTargetUpdateCommand) CommandName() string {
	return "rt_permission_target_update"
}

func (ptuc *PermissionTargetUpdateCommand) Run() (err error) {
	return ptuc.PerformPermissionTargetCmd(true)
}
