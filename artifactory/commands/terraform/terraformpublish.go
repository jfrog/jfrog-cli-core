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
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io/fs"
	"os"
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
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}
	return filepath.WalkDir(pwd, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		pathIinfo, e := os.Lstat(path)
		if e != nil {
			return e
		}
		// Skip files and check only directories.
		if !pathIinfo.IsDir() {
			return nil
		}
		terraformModule, e := isTerraformModule(path)
		if e != nil {
			return e
		}

		if terraformModule {
			moduleName := info.Name()
			return tpc.doDeploy(path, moduleName, tpc.serverDetails)
		}
		return nil
	})
	//return nil
	//return tpc.doDeploy("/Users/gail/dev/v2/jfrog-cli/testdata/terraform/terraformproject/aws/asg", "moduleName", tpc.serverDetails)
}

// We identify a terraform module by the existing of '.tf' files inside the directory.
// isTerraformModule search for '.tf' file inside the directory and returns true it founds at least one.
func isTerraformModule(path string) (bool, error) {
	dirname := path + string(filepath.Separator)

	d, err := os.Open(dirname)
	if err != nil {
		return false, err
	}
	defer d.Close()

	files, err := d.Readdir(-1)
	if err != nil {
		return false, err
	}
	for _, file := range files {
		if file.Mode().IsRegular() {
			if filepath.Ext(file.Name()) == ".tf" {
				return true, nil
			}
		}
	}
	return false, nil
}
func (tpc *TerraformPublishCommand) doDeploy(path, moduleName string, artDetails *config.ServerDetails) error {
	servicesManager, err := utils.CreateServiceManager(artDetails, -1, false)
	if err != nil {
		return err
	}
	target, err := tpc.getPublishTarget(moduleName)
	if err != nil {
		return err
	}
	up := services.NewUploadParams()
	//up.CommonParams = &specutils.CommonParams{Pattern: filepath.Join(path, "*"), Target: target}
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}
	pattern := strings.TrimPrefix(pwd+"/", path)
	up.CommonParams = &specutils.CommonParams{Pattern: pattern, Target: target}
	up.Archive = "zip"
	up.Recursive = true
	up.Exclusions = []string{"*.git", "*.DS_Store"}
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

//func getModuleNameByCurDir() (string, error) {
//	pwd, err := os.Getwd()
//	if err != nil {
//		return "", err
//	}
//	return filepath.Base(pwd), nil
//}

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
