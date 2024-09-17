package terraform

import (
	"errors"
	buildInfo "github.com/jfrog/build-info-go/entities"
	ioutils "github.com/jfrog/gofrog/io"
	"github.com/jfrog/gofrog/parallel"
	commandsUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-cli-core/v2/common/project"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	servicesUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"golang.org/x/exp/slices"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const threads = 3

type TerraformPublishCommandArgs struct {
	namespace          string
	provider           string
	tag                string
	exclusions         []string
	buildConfiguration *build.BuildConfiguration
	collectBuildInfo   bool
	buildProps         string
}

type TerraformPublishCommand struct {
	*TerraformPublishCommandArgs
	args           []string
	repo           string
	configFilePath string
	serverDetails  *config.ServerDetails
	result         *commandsUtils.Result
}

func NewTerraformPublishCommand() *TerraformPublishCommand {
	return &TerraformPublishCommand{TerraformPublishCommandArgs: NewTerraformPublishCommandArgs(), result: new(commandsUtils.Result)}
}

func NewTerraformPublishCommandArgs() *TerraformPublishCommandArgs {
	return &TerraformPublishCommandArgs{}
}

func (tpc *TerraformPublishCommand) SetArgs(terraformArg []string) *TerraformPublishCommand {
	tpc.args = terraformArg
	return tpc
}

func (tpc *TerraformPublishCommand) setServerDetails(serverDetails *config.ServerDetails) {
	tpc.serverDetails = serverDetails
}

func (tpc *TerraformPublishCommand) setRepoConfig(conf *project.RepositoryConfig) *TerraformPublishCommand {
	serverDetails, _ := conf.ServerDetails()
	tpc.setRepo(conf.TargetRepo()).setServerDetails(serverDetails)
	return tpc
}

func (tpc *TerraformPublishCommand) setRepo(repo string) *TerraformPublishCommand {
	tpc.repo = repo
	return tpc
}

func (tpc *TerraformPublishCommand) ServerDetails() (*config.ServerDetails, error) {
	return tpc.serverDetails, nil
}

func (tpc *TerraformPublishCommand) CommandName() string {
	return "rt_terraform_publish"
}

func (tpc *TerraformPublishCommand) SetConfigFilePath(configFilePath string) *TerraformPublishCommand {
	tpc.configFilePath = configFilePath
	return tpc
}

func (tpc *TerraformPublishCommand) Result() *commandsUtils.Result {
	return tpc.result
}

func (tpc *TerraformPublishCommand) Run() error {
	log.Info("Running Terraform publish")
	err := tpc.publish()
	if err != nil {
		return err
	}
	log.Info("Terraform publish finished successfully.")
	return nil
}

func (tpc *TerraformPublishCommand) Init() error {
	err := tpc.extractTerraformPublishOptionsFromArgs(tpc.args)
	if err != nil {
		return err
	}
	if tpc.namespace == "" || tpc.provider == "" || tpc.tag == "" {
		return errorutils.CheckErrorf("the --namespace, --provider and --tag options are mandatory")
	}
	if err = tpc.setRepoFromConfiguration(); err != nil {
		return err
	}
	artDetails, err := tpc.serverDetails.CreateArtAuthConfig()
	if err != nil {
		return err
	}
	if err = utils.ValidateRepoExists(tpc.repo, artDetails); err != nil {
		return err
	}
	tpc.collectBuildInfo, err = tpc.buildConfiguration.IsCollectBuildInfo()
	if err != nil {
		return err
	}
	if tpc.collectBuildInfo {
		tpc.buildProps, err = build.CreateBuildPropsFromConfiguration(tpc.buildConfiguration)
	}
	return err
}

func (tpc *TerraformPublishCommand) publish() error {
	log.Debug("Deploying terraform module...")
	success, failed, err := tpc.terraformPublish()
	if err != nil {
		return err
	}
	tpc.result.SetSuccessCount(success)
	tpc.result.SetFailCount(failed)
	return nil
}

func (tpa *TerraformPublishCommandArgs) extractTerraformPublishOptionsFromArgs(args []string) (err error) {
	// Extract namespace information from the args.
	var flagIndex, valueIndex int
	flagIndex, valueIndex, tpa.namespace, err = coreutils.FindFlag("--namespace", args)
	if err != nil {
		return
	}
	coreutils.RemoveFlagFromCommand(&args, flagIndex, valueIndex)
	// Extract provider information from the args.
	flagIndex, valueIndex, tpa.provider, err = coreutils.FindFlag("--provider", args)
	if err != nil {
		return
	}
	coreutils.RemoveFlagFromCommand(&args, flagIndex, valueIndex)
	// Extract tag information from the args.
	flagIndex, valueIndex, tpa.tag, err = coreutils.FindFlag("--tag", args)
	if err != nil {
		return
	}
	coreutils.RemoveFlagFromCommand(&args, flagIndex, valueIndex)
	// Extract exclusions information from the args.
	flagIndex, valueIndex, exclusionsString, err := coreutils.FindFlag("--exclusions", args)
	if err != nil {
		return
	}
	tpa.exclusions = append(tpa.exclusions, strings.Split(exclusionsString, ";")...)
	coreutils.RemoveFlagFromCommand(&args, flagIndex, valueIndex)
	args, tpa.buildConfiguration, err = build.ExtractBuildDetailsFromArgs(args)
	if err != nil {
		return err
	}
	if len(args) != 0 {
		err = errorutils.CheckErrorf("Unknown flag:" + strings.Split(args[0], "=")[0] + ". for a terraform publish command please provide --namespace, --provider, --tag and optionally --exclusions.")
	}
	return
}

func (tpc *TerraformPublishCommand) terraformPublish() (int, int, error) {
	uploadSummary := getNewUploadSummaryMultiArray()
	producerConsumer := parallel.NewRunner(3, 20000, false)
	errorsQueue := clientUtils.NewErrorsQueue(threads)

	tpc.prepareTerraformPublishTasks(producerConsumer, errorsQueue, uploadSummary)
	tpc.performTerraformPublishTasks(producerConsumer)
	e := errorsQueue.GetError()
	if e != nil {
		return 0, 0, e
	}
	return tpc.aggregateSummaryResults(uploadSummary)
}

func (tpc *TerraformPublishCommand) prepareTerraformPublishTasks(producer parallel.Runner, errorsQueue *clientUtils.ErrorsQueue, uploadSummary *[][]*servicesUtils.OperationSummary) {
	go func() {
		defer producer.Done()
		pwd, err := os.Getwd()
		if err != nil {
			log.Error(err)
			errorsQueue.AddError(err)
		}
		// Walk and upload directories which contain '.tf' files.
		err = tpc.walkDirAndUploadTerraformModules(pwd, producer, errorsQueue, uploadSummary, addTaskWithError)
		if err != nil && err != io.EOF {
			log.Error(err)
			errorsQueue.AddError(err)
		}
	}()
}

// ProduceTaskFunc is provided as an argument to 'walkDirAndUploadTerraformModules' function for testing purposes.
type ProduceTaskFunc func(producer parallel.Runner, serverDetails *config.ServerDetails, uploadSummary *[][]*servicesUtils.OperationSummary, uploadParams *services.UploadParams, errorsQueue *clientUtils.ErrorsQueue) (int, error)

func addTaskWithError(producer parallel.Runner, serverDetails *config.ServerDetails, uploadSummary *[][]*servicesUtils.OperationSummary, uploadParams *services.UploadParams, errorsQueue *clientUtils.ErrorsQueue) (int, error) {
	return producer.AddTaskWithError(uploadModuleTask(serverDetails, uploadSummary, uploadParams), errorsQueue.AddError)
}

func uploadModuleTask(serverDetails *config.ServerDetails, uploadSummary *[][]*servicesUtils.OperationSummary, uploadParams *services.UploadParams) parallel.TaskFunc {
	return func(threadId int) (err error) {
		summary, err := createServiceManagerAndUpload(serverDetails, uploadParams, false)
		if err != nil {
			return err
		}
		// Add summary to the thread's summary array.
		(*uploadSummary)[threadId] = append((*uploadSummary)[threadId], summary)
		return nil
	}
}

func createServiceManagerAndUpload(serverDetails *config.ServerDetails, uploadParams *services.UploadParams, dryRun bool) (operationSummary *servicesUtils.OperationSummary, err error) {
	serviceManager, err := utils.CreateServiceManagerWithThreads(serverDetails, dryRun, 1, -1, 0)
	if err != nil {
		return nil, err
	}
	return serviceManager.UploadFilesWithSummary(artifactory.UploadServiceOptions{}, *uploadParams)
}

func (tpc *TerraformPublishCommand) walkDirAndUploadTerraformModules(pwd string, producer parallel.Runner, errorsQueue *clientUtils.ErrorsQueue, uploadSummary *[][]*servicesUtils.OperationSummary, produceTaskFunc ProduceTaskFunc) error {
	return filepath.WalkDir(pwd, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		pathInfo, e := os.Lstat(path)
		if e != nil {
			return errorutils.CheckError(e)
		}
		// Skip files and check only directories.
		if !pathInfo.IsDir() {
			return nil
		}
		isTerraformModule, e := checkIfTerraformModule(path)
		if e != nil {
			return e
		}
		if isTerraformModule {
			uploadParams := tpc.uploadParamsForTerraformPublish(pathInfo.Name(), strings.TrimPrefix(path, pwd+string(filepath.Separator)))
			_, e = produceTaskFunc(producer, tpc.serverDetails, uploadSummary, uploadParams, errorsQueue)
			if e != nil {
				log.Error(e)
				errorsQueue.AddError(e)
			}

			// SkipDir will not stop the walk, but it will make us jump to the next directory.
			return filepath.SkipDir
		}
		return nil
	})
}

func (tpc *TerraformPublishCommand) performTerraformPublishTasks(consumer parallel.Runner) {
	// Blocking until consuming is finished.
	consumer.Run()
}

// Aggregate the operation summaries from all threads, to get the number of successful and failed uploads.
// If collecting build info, also aggregate the artifacts uploaded.
func (tpc *TerraformPublishCommand) aggregateSummaryResults(uploadSummary *[][]*servicesUtils.OperationSummary) (totalUploaded, totalFailed int, err error) {
	var artifacts []buildInfo.Artifact
	for i := 0; i < threads; i++ {
		threadSummary := (*uploadSummary)[i]
		for j := range threadSummary {
			// Operation summary should always be returned.
			if threadSummary[j] == nil {
				return 0, 0, errorutils.CheckErrorf("unexpected nil operation summary")
			}
			totalUploaded += threadSummary[j].TotalSucceeded
			totalFailed += threadSummary[j].TotalFailed

			if tpc.collectBuildInfo {
				buildArtifacts, err := readArtifactsFromSummary(threadSummary[j])
				if err != nil {
					return 0, 0, err
				}
				artifacts = append(artifacts, buildArtifacts...)
			}
		}
	}
	if tpc.collectBuildInfo {
		err = build.PopulateBuildArtifactsAsPartials(artifacts, tpc.buildConfiguration, buildInfo.Terraform)
	}
	return
}

func readArtifactsFromSummary(summary *servicesUtils.OperationSummary) (artifacts []buildInfo.Artifact, err error) {
	artifactsDetailsReader := summary.ArtifactsDetailsReader
	if artifactsDetailsReader == nil {
		return []buildInfo.Artifact{}, nil
	}
	defer ioutils.Close(artifactsDetailsReader, &err)
	return servicesUtils.ConvertArtifactsDetailsToBuildInfoArtifacts(artifactsDetailsReader)
}

func (tpc *TerraformPublishCommand) uploadParamsForTerraformPublish(moduleName, dirPath string) *services.UploadParams {
	uploadParams := services.NewUploadParams()
	uploadParams.Target = tpc.getPublishTarget(moduleName)
	uploadParams.Pattern = dirPath + "/(*)"
	uploadParams.TargetPathInArchive = "{1}"
	uploadParams.Archive = "zip"
	uploadParams.Recursive = true
	uploadParams.CommonParams.TargetProps = servicesUtils.NewProperties()
	uploadParams.CommonParams.Exclusions = append(slices.Clone(tpc.exclusions), "*.git", "*.DS_Store")
	uploadParams.BuildProps = tpc.buildProps
	return &uploadParams
}

// Module's path in terraform repository : namespace/moduleName/provider/tag.zip
func (tpc *TerraformPublishCommand) getPublishTarget(moduleName string) string {
	return path.Join(tpc.repo, tpc.namespace, moduleName, tpc.provider, tpc.tag+".zip")
}

// We identify a Terraform module by having at least one file with a ".tf" extension inside the module directory.
func checkIfTerraformModule(path string) (isModule bool, err error) {
	dirname := path + string(filepath.Separator)
	d, err := os.Open(dirname)
	if err != nil {
		return false, errorutils.CheckError(err)
	}
	defer func() {
		err = errors.Join(err, d.Close())
	}()

	files, err := d.Readdir(-1)
	if err != nil {
		return false, errorutils.CheckError(err)
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

func (tpc *TerraformPublishCommand) setRepoFromConfiguration() error {
	// Read config file.
	log.Debug("Preparing to read the config file", tpc.configFilePath)
	vConfig, err := project.ReadConfigFile(tpc.configFilePath, project.YAML)
	if err != nil {
		return err
	}
	deployerParams, err := project.GetRepoConfigByPrefix(tpc.configFilePath, project.ProjectConfigDeployerPrefix, vConfig)
	if err != nil {
		return err
	}
	tpc.setRepoConfig(deployerParams)
	return nil
}

// Each thread will save the summary of all its operations, in an array at its corresponding index.
func getNewUploadSummaryMultiArray() *[][]*servicesUtils.OperationSummary {
	uploadSummary := make([][]*servicesUtils.OperationSummary, threads)
	return &uploadSummary
}
