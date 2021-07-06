package permissiontarget

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

type PermissionTargetCreateCommand struct {
	PermissionTargetCommand
}

func NewPermissionTargetCreateCommand() *PermissionTargetCreateCommand {
	return &PermissionTargetCreateCommand{}
}

func (ptcc *PermissionTargetCreateCommand) SetTemplatePath(path string) *PermissionTargetCreateCommand {
	ptcc.templatePath = path
	return ptcc
}

func (ptcc *PermissionTargetCreateCommand) SetVars(vars string) *PermissionTargetCreateCommand {
	ptcc.vars = vars
	return ptcc
}

func (ptcc *PermissionTargetCreateCommand) SetServerDetails(serverDetails *config.ServerDetails) *PermissionTargetCreateCommand {
	ptcc.serverDetails = serverDetails
	return ptcc
}

func (ptcc *PermissionTargetCreateCommand) ServerDetails() (*config.ServerDetails, error) {
	return ptcc.serverDetails, nil
}

func (ptcc *PermissionTargetCreateCommand) CommandName() string {
	return "rt_permission_target_create"
}

func (ptcc *PermissionTargetCreateCommand) Run() (err error) {
	return ptcc.PerformPermissionTargetCmd(false)
}
