package generic

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	clientConfig "github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type PropsCommand struct {
	props   string
	threads int
	GenericCommand
}

func NewPropsCommand() *PropsCommand {
	return &PropsCommand{GenericCommand: *NewGenericCommand()}
}

func (pc *PropsCommand) Threads() int {
	return pc.threads
}

func (pc *PropsCommand) SetThreads(threads int) *PropsCommand {
	pc.threads = threads
	return pc
}

func (pc *PropsCommand) Props() string {
	return pc.props
}

func (pc *PropsCommand) SetProps(props string) *PropsCommand {
	pc.props = props
	return pc
}

func createPropsServiceManager(threads, httpRetries int, serverDetails *config.ServerDetails) (artifactory.ArtifactoryServicesManager, error) {
	certsPath, err := coreutils.GetJfrogCertsDir()
	if err != nil {
		return nil, err
	}
	artAuth, err := serverDetails.CreateArtAuthConfig()
	if err != nil {
		return nil, err
	}
	serviceConfig, err := clientConfig.NewConfigBuilder().
		SetServiceDetails(artAuth).
		SetCertificatesPath(certsPath).
		SetInsecureTls(serverDetails.InsecureTls).
		SetThreads(threads).
		SetHttpRetries(httpRetries).
		Build()

	return artifactory.New(serviceConfig)
}

func searchItems(spec *spec.SpecFiles, servicesManager artifactory.ArtifactoryServicesManager) (resultReader *content.ContentReader, err error) {
	var errorOccurred = false
	temp := []*content.ContentReader{}
	defer func() {
		for _, reader := range temp {
			reader.Close()
		}
	}()
	for i := 0; i < len(spec.Files); i++ {
		searchParams, err := utils.GetSearchParams(spec.Get(i))
		if err != nil {
			errorOccurred = true
			log.Error(err)
			continue
		}
		reader, err := servicesManager.SearchFiles(searchParams)
		if err != nil {
			errorOccurred = true
			log.Error(err)
			continue
		}
		temp = append(temp, reader)
	}
	resultReader, err = content.MergeReaders(temp, content.DefaultKey)
	if err != nil {
		return
	}
	if errorOccurred {
		err = errorutils.CheckErrorf("Operation finished with errors, please review the logs.")
	}
	return
}

func GetPropsParams(reader *content.ContentReader, properties string) (propsParams services.PropsParams) {
	propsParams = services.NewPropsParams()
	propsParams.Reader = reader
	propsParams.Props = properties
	return
}
