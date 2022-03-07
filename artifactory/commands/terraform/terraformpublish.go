package terraform

import (
	"github.com/jfrog/gofrog/parallel"
	commandsUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	servicesUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

const threads = 3

type TerraformPublishCommandArgs struct {
	namespace  string
	moduleName string
	provider   string
	tag        string
	exclusions []string
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

func (tpc *TerraformPublishCommand) GetArgs() []string {
	return tpc.args
}

func (tpc *TerraformPublishCommand) SetArgs(terraformArg []string) *TerraformPublishCommand {
	tpc.args = terraformArg
	return tpc
}

func (tpc *TerraformPublishCommand) SetServerDetails(serverDetails *config.ServerDetails) *TerraformPublishCommand {
	tpc.serverDetails = serverDetails
	return tpc
}

func (tpc *TerraformPublishCommand) SetRepo(repo string) *TerraformPublishCommand {
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

func (tpc *TerraformPublishCommand) SetModuleName(moduleName string) *TerraformPublishCommand {
	tpc.moduleName = moduleName
	return tpc
}

func (tpc *TerraformPublishCommand) Run() error {
	log.Info("Running Terraform publish")
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
	err := tpc.extractTerraformPublishOptionsFromArgs(tpc.args)
	if err != nil {
		return err
	}
	if tpc.namespace == "" || tpc.provider == "" || tpc.tag == "" {
		return errorutils.CheckErrorf("The --namespace, --provider and --tag options are mandatory.")
	}
	if err := tpc.setRepoFromConfiguration(); err != nil {
		return err
	}
	artDetails, err := tpc.serverDetails.CreateArtAuthConfig()
	if err != nil {
		return err
	}
	return utils.ValidateRepoExists(tpc.repo, artDetails)
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
	if len(args) != 0 {
		err = errorutils.CheckErrorf("Unknown flag:" + strings.Split(args[0], "=")[0] + ". for a terraform publish command please provide --namespace, --provider, --tag and optionally --exclusions.")
	}
	return
}

func (tpc *TerraformPublishCommand) terraformPublish() (int, int, error) {
	uploadSummary := servicesUtils.NewResult(threads)
	producerConsumer := parallel.NewRunner(3, 20000, false)
	errorsQueue := clientutils.NewErrorsQueue(threads)

	tpc.prepareTerraformPublishTasks(producerConsumer, errorsQueue, uploadSummary)
	totalUploaded, totalFailed := tpc.performTerraformPublishTasks(producerConsumer, uploadSummary)
	e := errorsQueue.GetError()
	if e != nil {
		return 0, 0, e
	}
	return totalUploaded, totalFailed, nil
}

func (tpc *TerraformPublishCommand) prepareTerraformPublishTasks(producer parallel.Runner, errorsQueue *clientutils.ErrorsQueue, uploadSummary *servicesUtils.Result) {
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

// ProduceTaskFunk is provided as an argument to 'walkDirAndUploadTerraformModules' function for testing purposes.
type ProduceTaskFunk func(producer parallel.Runner, uploadService *services.UploadService, uploadSummary *servicesUtils.Result, target string, archiveData *services.ArchiveUploadData, errorsQueue *clientutils.ErrorsQueue) (int, error)

func addTaskWithError(producer parallel.Runner, uploadService *services.UploadService, uploadSummary *servicesUtils.Result, target string, archiveData *services.ArchiveUploadData, errorsQueue *clientutils.ErrorsQueue) (int, error) {
	return producer.AddTaskWithError(uploadService.CreateUploadAsZipFunc(uploadSummary, target, archiveData, errorsQueue), errorsQueue.AddError)
}

func (tpc *TerraformPublishCommand) walkDirAndUploadTerraformModules(pwd string, producer parallel.Runner, errorsQueue *clientutils.ErrorsQueue, uploadSummary *servicesUtils.Result, produceTaskFunk ProduceTaskFunk) error {
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
			archiveData := createTerraformArchiveUploadData(uploadParams, errorsQueue)
			dataHandlerFunc := GetSaveTaskInContentWriterFunc(archiveData, *uploadParams, errorsQueue)
			// Collect files that matches uploadParams and write them in archiveData's writer.
			e = services.CollectFilesForUpload(*uploadParams, nil, nil, dataHandlerFunc)
			if e != nil {
				log.Error(e)
				errorsQueue.AddError(e)
			}
			e = archiveData.GetWriter().Close()
			if e != nil {
				log.Error(e)
				errorsQueue.AddError(e)
			}
			// In case all files were excluded and no files were written to writer- skip this module.
			if archiveData.GetWriter().IsEmpty() {
				return filepath.SkipDir
			}
			uploadService := createUploadServiceManager(tpc.serverDetails, errorsQueue)
			_, e = produceTaskFunk(producer, uploadService, uploadSummary, uploadParams.Target, archiveData, errorsQueue)
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

func createTerraformArchiveUploadData(uploadParams *services.UploadParams, errorsQueue *clientutils.ErrorsQueue) *services.ArchiveUploadData {
	archiveData := services.ArchiveUploadData{}
	var err error
	archiveData.SetUploadParams(services.DeepCopyUploadParams(uploadParams))
	writer, err := content.NewContentWriter("archive", true, false)
	if err != nil {
		log.Error(err)
		errorsQueue.AddError(err)
	}
	archiveData.SetWriter(writer)
	return &archiveData
}

func createUploadServiceManager(serverDetails *config.ServerDetails, errorsQueue *clientutils.ErrorsQueue) *services.UploadService {
	// Upload module using upload service
	serviceManager, err := utils.CreateServiceManager(serverDetails, 0, 0, false)
	if err != nil {
		log.Error(err)
		errorsQueue.AddError(err)
	}
	uploadService := services.NewUploadService(serviceManager.Client())
	uploadService.ArtDetails = serviceManager.GetConfig().GetServiceDetails()
	uploadService.Threads = serviceManager.GetConfig().GetThreads()
	return uploadService
}

func GetSaveTaskInContentWriterFunc(archiveData *services.ArchiveUploadData, uploadParams services.UploadParams, errorsQueue *clientutils.ErrorsQueue) services.UploadDataHandlerFunc {
	return func(data services.UploadData) {
		archiveData.GetWriter().Write(data)
	}
}

func (tpc *TerraformPublishCommand) performTerraformPublishTasks(consumer parallel.Runner, uploadSummary *servicesUtils.Result) (totalUploaded, totalFailed int) {
	// Blocking until consuming is finished.
	consumer.Run()
	totalUploaded = servicesUtils.SumIntArray(uploadSummary.SuccessCount)
	totalUploadAttempted := servicesUtils.SumIntArray(uploadSummary.TotalCount)

	log.Debug("Uploaded", strconv.Itoa(totalUploaded), "artifacts.")
	totalFailed = totalUploadAttempted - totalUploaded
	if totalFailed > 0 {
		log.Error("Failed uploading", strconv.Itoa(totalFailed), "artifacts.")
	}
	return
}

func (tpc *TerraformPublishCommand) uploadParamsForTerraformPublish(moduleName, dirPath string) *services.UploadParams {
	uploadParams := services.NewUploadParams()
	uploadParams.Target = tpc.getPublishTarget(moduleName)
	uploadParams.Pattern = dirPath + "/(*)"
	uploadParams.TargetPathInArchive = "{1}"
	uploadParams.Archive = "zip"
	uploadParams.Recursive = true
	uploadParams.CommonParams.TargetProps = servicesUtils.NewProperties()
	uploadParams.CommonParams.Exclusions = append(tpc.exclusions, "*.git", "*.DS_Store")

	return &uploadParams
}

// Module's path in terraform repository : namespace/provider/moduleName/tag.zip
func (tpc *TerraformPublishCommand) getPublishTarget(moduleName string) string {
	return path.Join(tpc.repo, tpc.namespace, tpc.provider, moduleName, tpc.tag+".zip")
}

// We identify a Terraform module by the existence of a file with a ".tf" extension inside the module directory.
// isTerraformModule search for a file with a ".tf" extension inside and returns true it founds at least one.
func checkIfTerraformModule(path string) (isModule bool, err error) {
	dirname := path + string(filepath.Separator)
	d, err := os.Open(dirname)
	if err != nil {
		return false, errorutils.CheckError(err)
	}
	defer func() {
		e := d.Close()
		if err == nil {
			err = errorutils.CheckError(e)
		}
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
