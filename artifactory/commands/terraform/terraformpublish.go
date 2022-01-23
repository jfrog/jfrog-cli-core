package terraform

import (
	"github.com/jfrog/gofrog/parallel"
	commandsutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	commandutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli/utils/cliutils"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	clientservicesutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	specutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
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
	return utils.ValidateRepoExists(tpc.repo, artDetails)
}

func (tpc *TerraformPublishCommand) publish() error {
	log.Debug("Deploying terraform module.")
	success, failed, err := tpc.TerraformPublish()
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

func (ts *TerraformPublishCommand) TerraformPublish() (int, int, error) {
	var e error
	uploadSummary := clientservicesutils.NewResult(cliutils.Threads)
	producerConsumer := parallel.NewRunner(cliutils.Threads, 20000, false)
	errorsQueue := clientutils.NewErrorsQueue(1)

	ts.prepareTerraformPublishTasks(producerConsumer, errorsQueue, uploadSummary)
	totalUploaded, totalFailed := ts.performTerraformPublishTasks(producerConsumer, uploadSummary)
	e = errorsQueue.GetError()
	if e != nil {
		return 0, 0, e
	}
	return totalUploaded, totalFailed, nil
}

func (ts *TerraformPublishCommand) prepareTerraformPublishTasks(producer parallel.Runner, errorsQueue *clientutils.ErrorsQueue, uploadSummary *clientservicesutils.Result) {
	go func() {
		defer producer.Done()
		toArchive := make(map[string]*services.ArchiveUploadData)
		pwd, err := os.Getwd()
		if err != nil {
			log.Error(err)
			errorsQueue.AddError(err)
		}
		// Walk and upload directories which contain '.tf' files.
		err = filepath.WalkDir(pwd, func(path string, info fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			pathInfo, e := os.Lstat(path)
			if e != nil {
				return e
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
				uploadParams, e := ts.uploadParamsForTerraformPublish(pathInfo.Name(), strings.TrimPrefix(path, pwd+string(filepath.Separator)))
				if e != nil {
					return e
				}
				dataHandlerFunc := services.GetSaveTaskInContentWriterFunc(toArchive, *uploadParams, errorsQueue)
				e = services.CollectFilesForUpload(*uploadParams, nil, nil, dataHandlerFunc)
				if e != nil {
					return e
				}
				// SkipDir will not stop the walk, but will jump to the next directory.
				return filepath.SkipDir
			}
			return nil
		})
		if err != nil && err != io.EOF {
			log.Error(err)
			errorsQueue.AddError(err)
		}

		// Upload modules
		for targetPath, archiveData := range toArchive {
			err := archiveData.GetWriter().Close()
			if err != nil {
				log.Error(err)
				errorsQueue.AddError(err)
			}
			// Upload module using upload service
			serviceManager, err := utils.CreateServiceManager(ts.serverDetails, 0, 0, false)
			if err != nil {
				log.Error(err)
				errorsQueue.AddError(err)
			}
			uploadService := services.NewUploadService(serviceManager.Client())
			uploadService.ArtDetails = serviceManager.GetConfig().GetServiceDetails()
			uploadService.Threads = serviceManager.GetConfig().GetThreads()
			producer.AddTaskWithError(uploadService.CreateUploadAsZipFunc(uploadSummary, targetPath, archiveData, errorsQueue), errorsQueue.AddError)
		}
	}()
}

func (ts *TerraformPublishCommand) performTerraformPublishTasks(consumer parallel.Runner, uploadSummary *clientservicesutils.Result) (totalUploaded, totalFailed int) {
	// Blocking until consuming is finished.
	consumer.Run()
	totalUploaded = clientservicesutils.SumIntArray(uploadSummary.SuccessCount)
	totalUploadAttempted := clientservicesutils.SumIntArray(uploadSummary.TotalCount)

	log.Debug("Uploaded", strconv.Itoa(totalUploaded), "artifacts.")
	totalFailed = totalUploadAttempted - totalUploaded
	if totalFailed > 0 {
		log.Error("Failed uploading", strconv.Itoa(totalFailed), "artifacts.")
	}
	return
}

func (tp *TerraformPublishCommand) uploadParamsForTerraformPublish(moduleName, dirPath string) (*services.UploadParams, error) {
	uploadParams := services.NewUploadParams()
	uploadParams.Target = tp.getPublishTarget(moduleName)
	uploadParams.Pattern = dirPath + "/(*)"
	uploadParams.TargetPathInArchive = "{1}"
	uploadParams.Archive = "zip"
	uploadParams.Recursive = true
	uploadParams.CommonParams.TargetProps = specutils.NewProperties()
	uploadParams.CommonParams.Exclusions = append(tp.exclusions, "*.git", "*.DS_Store")

	return &uploadParams, nil
}

// Module's path in terraform repository : namespace/provider/moduleName/tag.zip
func (tp *TerraformPublishCommand) getPublishTarget(moduleName string) string {
	return filepath.ToSlash(filepath.Join(tp.repo, tp.namespace, tp.provider, moduleName, tp.tag+".zip"))
}

// We identify a Terraform module by the existing of a '.tf' file inside the module directory.
// isTerraformModule search for '.tf' file inside and returns true it founds at least one.
func checkIfTerraformModule(path string) (bool, error) {
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
