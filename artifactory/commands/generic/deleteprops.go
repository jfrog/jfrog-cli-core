package generic

import (
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

type DeletePropsCommand struct {
	PropsCommand
}

func NewDeletePropsCommand() *DeletePropsCommand {
	return &DeletePropsCommand{}
}

func (dp *DeletePropsCommand) DeletePropsCommand(command PropsCommand) *DeletePropsCommand {
	dp.PropsCommand = command
	return dp
}

func (dp *DeletePropsCommand) CommandName() string {
	return "rt_delete_properties"
}

func (dp *DeletePropsCommand) Run() error {
	serverDetails, err := dp.ServerDetails()
	if errorutils.CheckError(err) != nil {
		return err
	}
	servicesManager, err := createPropsServiceManager(dp.threads, dp.retries, dp.retryWaitTimeMilliSecs, serverDetails)
	if err != nil {
		return err
	}
	reader, err := searchItems(dp.Spec(), servicesManager)
	if err != nil {
		return err
	}
	defer reader.Close()
	propsParams := GetPropsParams(reader, dp.props)
	success, err := servicesManager.DeleteProps(propsParams)
	result := dp.Result()
	result.SetSuccessCount(success)
	totalLength, totalLengthErr := reader.Length()
	result.SetFailCount(totalLength - success)
	if totalLengthErr != nil {
		return totalLengthErr
	}
	return err
}
