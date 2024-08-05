package generic

import (
	"errors"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

type SetPropsCommand struct {
	PropsCommand
}

func NewSetPropsCommand() *SetPropsCommand {
	return &SetPropsCommand{}
}

func (setProps *SetPropsCommand) SetPropsCommand(command PropsCommand) *SetPropsCommand {
	setProps.PropsCommand = command
	return setProps
}

func (setProps *SetPropsCommand) CommandName() string {
	return "rt_set_properties"
}

func (setProps *SetPropsCommand) Run() (err error) {
	serverDetails, err := setProps.ServerDetails()
	if errorutils.CheckError(err) != nil {
		return err
	}
	servicesManager, err := createPropsServiceManager(setProps.threads, setProps.retries, setProps.retryWaitTimeMilliSecs, serverDetails)
	if err != nil {
		return err
	}

	reader, err := searchItems(setProps.Spec(), servicesManager)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, reader.Close())
	}()
	propsParams := GetPropsParams(reader, setProps.props)
	success, err := servicesManager.SetProps(propsParams)

	result := setProps.Result()
	result.SetSuccessCount(success)
	totalLength, totalLengthErr := reader.Length()
	result.SetFailCount(totalLength - success)
	if totalLengthErr != nil {
		return totalLengthErr
	}
	return err
}
