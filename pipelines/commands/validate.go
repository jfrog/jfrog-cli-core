package commands

import (
	"bytes"
	"encoding/json"
	"github.com/ghodss/yaml"
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/manager"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io"
	"os"
	"strings"
	"time"
)

type ValidateCommand struct {
	serverDetails *config.ServerDetails
	pathToFile    string
	data          []byte
}

type RespBody struct {
	IsValid bool               `json:"isValid"`
	Errors  []ValidationErrors `json:"errors"`
}

type ValidationErrors struct {
	Text       string
	LineNumber int
}

func NewValidateCommand() *ValidateCommand {
	return &ValidateCommand{}
}

func (vc *ValidateCommand) ServerDetails() (*config.ServerDetails, error) {
	return vc.serverDetails, nil
}

func (vc *ValidateCommand) SetServerDetails(serverDetails *config.ServerDetails) *ValidateCommand {
	vc.serverDetails = serverDetails
	return vc
}

func (vc *ValidateCommand) SetPipeResourceFiles(f string) *ValidateCommand {
	vc.pathToFile = f
	return vc
}

func (vc *ValidateCommand) CommandName() string {
	return "pl_validate"
}

func (vc *ValidateCommand) Run() error {
	serviceManager, err := manager.CreateServiceManager(vc.serverDetails)
	if err != nil {
		return err
	}
	data, err := vc.ValidateResources()
	if err != nil {
		return err
	}
	response, err := serviceManager.ValidatePipelineSources(data)
	log.Info(response)
	return err
}

func (vc *ValidateCommand) runValidation(resMap map[string]string) ([]byte, error) {
	var buf *bytes.Buffer
	payload, err := getPayloadToValidatePipelineResource(resMap)
	if err != nil {
		return []byte{}, err
	}
	if buf != nil {
		_, err := buf.Read(payload.Bytes())
		if err != nil {
			return []byte{}, err
		}
	} else {
		buf = payload
	}
	b := buf.Bytes()
	return b, nil
}

func (vc *ValidateCommand) ValidateResources() ([]byte, error) {
	log.Info("Pipeline resources found for processing ")
	ymlType := ""
	readFile, err := os.ReadFile(vc.pathToFile)
	if err != nil {
		return []byte{}, err
	}
	fileInfo, err := os.Stat(vc.pathToFile)
	if err != nil {
		return []byte{}, err
	}
	toJSON, err := convertYAMLToJSON(err, readFile)
	if err != nil {
		return []byte{}, err
	}
	vsc, err := convertJSONDataToMap(fileInfo, toJSON)
	if err != nil {
		return []byte{}, err
	}
	resMap, err, done := splitDataToPipelinesAndResourcesMap(vsc, ymlType)
	if done {
		return []byte{}, err
	}
	data, err := vc.runValidation(resMap)
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

func convertJSONDataToMap(file os.FileInfo, toJSON []byte) (map[string][]interface{}, error) {
	log.Info("Validating pipeline resources ", file.Name())
	time.Sleep(1 * time.Second)
	vsc := make(map[string][]interface{})
	err := yaml.Unmarshal(toJSON, &vsc)
	if err != nil {
		return nil, err
	}
	return vsc, nil
}

func convertYAMLToJSON(err error, readFile []byte) ([]byte, error) {
	toJSON, err := yaml.YAMLToJSON(readFile)
	if err != nil {
		log.Error("Failed to convert to json")
		return nil, err
	}
	return toJSON, nil
}

func splitDataToPipelinesAndResourcesMap(vsc map[string][]interface{}, ymlType string) (map[string]string, error, bool) {
	resMap := make(map[string]string)
	if v, ok := vsc["resources"]; ok {
		log.Info("Resources found preparing to validate")
		data, err := json.Marshal(v)
		if err != nil {
			log.Error("Failed to marshal to json")
			return nil, err, true
		}
		ymlType = "resources"
		resMap[ymlType] = string(data)
	}
	if vp, ok := vsc["pipelines"]; ok {
		log.Info("Pipelines found preparing to validate")
		data, err := json.Marshal(vp)
		if err != nil {
			log.Error("Failed to marshal to json")
			return nil, err, true
		}
		ymlType = "pipelines"
		resMap[ymlType] = string(data)
	}
	return resMap, nil, false
}

func getPayloadToValidatePipelineResource(resMap map[string]string) (*bytes.Buffer, error) {
	payload := getPayloadBasedOnYmlType(resMap)
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(payload)
	if err != nil {
		log.Error("Failed to read stream to send payload to trigger pipelines")
		return nil, err
	}
	return buf, err
}

func getPayloadBasedOnYmlType(m map[string]string) *strings.Reader {
	var resReader, pipReader, valReader *strings.Reader
	for ymlType := range m {
		if ymlType == "resources" {
			resReader = strings.NewReader(`{"fileName":"` + ymlType + `.yml","content":` + m[ymlType] + `,"ymlType":"` + ymlType + `"}`)
		} else if ymlType == "pipelines" {
			pipReader = strings.NewReader(`{"fileName":"` + ymlType + `.yml","content":` + m[ymlType] + `,"ymlType":"` + ymlType + `"}`)
		}
	}
	if resReader != nil && pipReader != nil {
		resAll, err := io.ReadAll(resReader)
		if err != nil {
			return nil
		}
		pipAll, err := io.ReadAll(pipReader)
		if err != nil {
			return nil
		}
		valReader = strings.NewReader(`{"files":[` + string(resAll) + `,` + string(pipAll) + `]}`)
	} else if resReader != nil {
		all, err := io.ReadAll(resReader)
		if err != nil {
			return nil
		}
		valReader = strings.NewReader(`{"files":[` + string(all) + `]}`)
	} else if pipReader != nil {
		all, err := io.ReadAll(pipReader)
		if err != nil {
			return nil
		}
		valReader = strings.NewReader(`{"files":[` + string(all) + `]}`)
	}
	return valReader
}
