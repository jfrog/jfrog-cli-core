package terraform

import (
	commandsutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	commandutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	specutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	utils2 "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"path/filepath"
	"strings"
)

type TerraformPublishCommandArgs struct {
	TerraformCommand
	artifactsDetailsReader *content.ContentReader
	namespace              string
	moduleName             string
	provider               string
	tag                    string
}

type TerraformPublishCommand struct {
	configFilePath string
	commandName    string
	result         *commandsutils.Result
	*TerraformPublishCommandArgs
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

func (tpc *TerraformPublishCommand) Run() error {
	log.Info("Running Terraform Publish")
	if err := tpc.preparePrerequisites(); err != nil {
		return err
	}
	if err := tpc.publish(); err != nil {
		return err
	}
	log.Info("Terraform publish finished successfully.")
	return nil
}

func (tpc *TerraformPublishCommand) preparePrerequisites() error {
	namespace, provider, tag, err := ExtractTerraformPublishOptionsFromArgs(tpc.args)
	if err != nil {
		return err
	}
	if namespace == "" || provider == "" || tag == "" {
		return errorutils.CheckErrorf("Wrong number of arguments. for a terraform publish command please provide --namespace, --provider and --tag.")
	}
	tpc.SetNamespace(namespace).SetProvider(provider).SetTag(tag)
	if err := tpc.getRepoFromConfiguration(); err != nil {
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
	servicesManager, err := utils.CreateTerraformServiceManager(tpc.serverDetails,0, false)
	if err != nil {
		return err
	}
	commonParams := specutils.CommonParams{TargetProps: specutils.NewProperties()}
	terraformParams := services.NewTerraformParams(&commonParams).SetTargetRepo(tpc.repo).SetNamespace(tpc.namespace).SetProvider(tpc.provider).SetTag(tpc.tag)
	success, failed, err := servicesManager.PublishTerraformModule(*terraformParams)
	if err != nil {
		return err
	}
	tpc.result.SetSuccessCount(success)
	tpc.result.SetFailCount(failed)
	//terraformService := services.NewTerraformService(servicesManager.Client(), servicesManager.GetConfig().GetServiceDetails())
	//
	//terraformService.TerraformPublish(terraformParams)
	return nil
	//pwd, err := os.Getwd()
	//if err != nil {
	//	return err
	//}
	//return filepath.WalkDir(pwd, func(path string, info fs.DirEntry, err error) error {
	//	if err != nil {
	//		return err
	//	}
	//	pathIinfo, e := os.Lstat(path)
	//	if e != nil {
	//		return e
	//	}
	//	// Skip files and check only directories.
	//	if !pathIinfo.IsDir() {
	//		return nil
	//	}
	//	terraformModule, e := isTerraformModule(path)
	//	if e != nil {
	//		return e
	//	}
	//
	//	if terraformModule {
	//		moduleName := info.Name()
	//		return tpc.doDeploy(path, moduleName, tpc.serverDetails)
	//	}
	//	return nil
	//})
	//return nil
	//return tpc.doDeploy("/Users/gail/dev/v2/jfrog-cli/testdata/terraform/terraformproject/aws/asg", "moduleName", tpc.serverDetails)
}


func (tpc *TerraformPublishCommand) getPublishTarget(moduleName string) (string, error) {
	return filepath.ToSlash(filepath.Join(tpc.repo, tpc.namespace, tpc.provider, moduleName, tpc.tag+".zip")), nil
}

func (tpc *TerraformPublishCommand) getRepoFromConfiguration() error {
	// Read config file.
	log.Debug("Preparing to read the config file", tpc.configFilePath)
	vConfig, err := utils.ReadConfigFile(tpc.configFilePath, utils.YAML)
	if err != nil {
		return err
	}
	deployerParams, err := utils.GetRepoConfigByPrefix(tpc.configFilePath, utils.ProjectConfigDeployerPrefix, vConfig)
	if err != nil {
		return err
	}
	tpc.SetRepo(deployerParams.TargetRepo())
	return nil
}

func ExtractTerraformPublishOptionsFromArgs(args []string) (namespace, provider, tag string, err error) {
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
	if len(args) != 0 {
		err = errorutils.CheckErrorf("Unknown flag:\"" + strings.Split(args[0], "=")[0] + "\". for a terraform publish command please provide --namespace, --provider and --tag.")
	}
	return
}

func (tpc *TerraformPublishCommand) doDeploy(path, moduleName string, artDetails *config.ServerDetails) (err error) {
	servicesManager, err := utils.CreateServiceManager(artDetails, -1, false)
	if err != nil {
		return err
	}
	target, err := tpc.getPublishTarget(moduleName)
	if err != nil {
		return err
	}
	up := services.NewUploadParams()
	up.CommonParams = &specutils.CommonParams{Pattern: "./*", Target: target}
	up.Archive = "zip"
	up.Recursive = true
	up.Exclusions = []string{"*.git", "*.DS_Store"}
	callbackFunc, err := utils2.ChangeDirWithCallback(path)
	if err != nil {
		return err
	}
	defer callbackFunc()
	totalSucceeded, totalFailed, err := servicesManager.UploadFiles(up)
	if err != nil {
		return err
	}
	tpc.result.SetFailCount(totalFailed + tpc.result.FailCount())
	tpc.result.SetSuccessCount(totalSucceeded + tpc.result.SuccessCount())

	// We deploying only one Artifact which have to be deployed, in case of failure we should fail
	if totalFailed > 0 {
		return errorutils.CheckErrorf("Failed to upload the terraform module to Artifactory. See Artifactory logs for more details.")
	}
	return nil
}
