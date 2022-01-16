package terraform

import (
	commandsutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	commandutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	specutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"strings"
)

type TerraformPublishCommandArgs struct {
	namespace  string
	moduleName string
	provider   string
	tag        string
	exclusions []string
}

type TerraformPublishCommand struct {
	TerraformCommand
	*TerraformPublishCommandArgs
	commandName string
	result      *commandsutils.Result
}

func NewTerraformPublishCommand() *TerraformPublishCommand {
	return &TerraformPublishCommand{TerraformPublishCommandArgs: NewTerraformPublishCommandArgs(), commandName: "rt_terraform_publish", result: new(commandsutils.Result)}
}

func NewTerraformPublishCommandArgs() *TerraformPublishCommandArgs {
	return &TerraformPublishCommandArgs{}
}

func (npc *TerraformPublishCommand) ServerDetails() (*config.ServerDetails, error) {
	return npc.serverDetails, nil
}

func (tpc *TerraformPublishCommand) CommandName() string {
	return tpc.commandName
}

func (tpc *TerraformPublishCommand) SetConfigFilePath(configFilePath string) *TerraformPublishCommand {
	tpc.configFilePath = configFilePath
	return tpc
}

func (tpc *TerraformPublishCommand) Result() *commandutils.Result {
	return tpc.result
}

func (tpc *TerraformPublishCommand) SetNamespace(namespace string) *TerraformPublishCommand {
	tpc.namespace = namespace
	return tpc
}

func (tpc *TerraformPublishCommand) SetModuleName(moduleName string) *TerraformPublishCommand {
	tpc.moduleName = moduleName
	return tpc
}

func (tpc *TerraformPublishCommand) SetProvider(provider string) *TerraformPublishCommand {
	tpc.provider = provider
	return tpc
}

func (tpc *TerraformPublishCommand) SetTag(tag string) *TerraformPublishCommand {
	tpc.tag = tag
	return tpc
}

func (tpc *TerraformPublishCommand) SetExclusions(exclusions []string) *TerraformPublishCommand {
	tpc.exclusions = exclusions
	return tpc
}

func (tpc *TerraformPublishCommand) Run() error {
	log.Info("Running Terraform Publish")
	err := tpc.preparePrerequisites()
	if err != nil {
		return err
	}
	err = tpc.publish()
	if err != nil {
		return err
	}
	log.Info("Terraform publish finished successfully.")
	return nil
}

func (tpc *TerraformPublishCommand) preparePrerequisites() error {
	namespace, provider, tag, exclusions, err := ExtractTerraformPublishOptionsFromArgs(tpc.args)
	if err != nil {
		return err
	}
	if namespace == "" || provider == "" || tag == "" {
		return errorutils.CheckErrorf("Wrong number of arguments. for a terraform publish command please provide --namespace, --provider and --tag.")
	}
	tpc.SetNamespace(namespace).SetProvider(provider).SetTag(tag).SetExclusions(exclusions)
	if err := tpc.setRepoFromConfiguration(); err != nil {
		return err
	}
	artDetails, err := tpc.serverDetails.CreateArtAuthConfig()
	if err != nil {
		return err
	}
	return utils.CheckIfRepoExists(tpc.repo, artDetails)
}

func (tpc *TerraformPublishCommand) publish() error {
	log.Debug("Deploying terraform module.")
	servicesManager, err := utils.CreateTerraformServiceManager(tpc.serverDetails, 0, 0, false)
	if err != nil {
		return err
	}
	commonParams := specutils.CommonParams{TargetProps: specutils.NewProperties(), Exclusions: tpc.exclusions}
	terraformParams := services.NewTerraformParams(&commonParams).SetTargetRepo(tpc.repo).SetNamespace(tpc.namespace).SetProvider(tpc.provider).SetTag(tpc.tag)
	success, failed, err := servicesManager.PublishTerraformModule(*terraformParams)
	if err != nil {
		return err
	}
	tpc.result.SetSuccessCount(success)
	tpc.result.SetFailCount(failed)
	return nil
}

func ExtractTerraformPublishOptionsFromArgs(args []string) (namespace, provider, tag string, exclusions []string, err error) {
	// Extract namespace information from the args.
	flagIndex, valueIndex, namespace, err := coreutils.FindFlag("--namespace", args)
	if err != nil {
		return
	}
	coreutils.RemoveFlagFromCommand(&args, flagIndex, valueIndex)
	// Extract provider information from the args.
	flagIndex, valueIndex, provider, err = coreutils.FindFlag("--provider", args)
	if err != nil {
		return
	}
	coreutils.RemoveFlagFromCommand(&args, flagIndex, valueIndex)
	// Extract tag information from the args.
	flagIndex, valueIndex, tag, err = coreutils.FindFlag("--tag", args)
	if err != nil {
		return
	}
	coreutils.RemoveFlagFromCommand(&args, flagIndex, valueIndex)
	// Extract exclusions information from the args.
	flagIndex, valueIndex, exclusionsString, err := coreutils.FindFlag("--exclusions", args)
	for _, singleValue := range strings.Split(exclusionsString, ";") {
		exclusions = append(exclusions, singleValue)
	}
	if err != nil {
		return
	}
	coreutils.RemoveFlagFromCommand(&args, flagIndex, valueIndex)
	if len(args) != 0 {
		err = errorutils.CheckErrorf("Unknown flag:\"" + strings.Split(args[0], "=")[0] + "\". for a terraform publish command please provide --namespace, --provider and --tag.")
	}
	return
}
