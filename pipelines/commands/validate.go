package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	yamlconv "github.com/ghodss/yaml"
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/manager"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	clientlog "github.com/jfrog/jfrog-client-go/utils/log"
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

func (vc *ValidateCommand) Run() (string, error) {
	serviceManager, err := manager.CreateServiceManager(vc.serverDetails)
	if err != nil {
		return "", err
	}
	data, validationErr := vc.ValidateResources()
	if validationErr != nil {
		return "", validationErr
	}
	return serviceManager.ValidatePipelineSources(data)
}

func (vc *ValidateCommand) runValidation(resMap map[string]string) ([]byte, error) {
	var buf *bytes.Buffer
	payload, valErr := getPayloadToValidatePipelineResource(resMap)
	if valErr != nil {
		return []byte{}, valErr
	}
	if buf != nil {
		_, readErr := buf.Read(payload.Bytes())
		if readErr != nil {
			return []byte{}, readErr
		}
	} else {
		buf = payload
	}
	b := buf.Bytes()
	//fmt.Printf("awesome data : %+v \n", string(b))
	return b, nil
}

func (vc *ValidateCommand) ValidateResources() ([]byte, error) {

	clientlog.Info("pipeline resources found for processing ")
	ymlType := ""

	readFile, err := os.ReadFile(vc.pathToFile)
	if err != nil {
		return []byte{}, err
	}
	fInfo, err := os.Stat(vc.pathToFile)
	if err != nil {
		return []byte{}, err
	}

	toJSON, err3 := convertYAMLToJSON(err, readFile)
	if err3 != nil {
		return []byte{}, err3
	}
	vsc, err4 := convertJSONDataToMap(fInfo, toJSON)
	if err4 != nil {
		return []byte{}, err4
	}
	var marErr error

	resMap, err5, done := splitDataToPipelinesAndResourcesMap(vsc, marErr, err, ymlType)
	if done {
		return []byte{}, err5
	}

	data, resErr := vc.runValidation(resMap)
	if resErr != nil {
		return []byte{}, resErr
	}

	return data, nil
}

func convertJSONDataToMap(file os.FileInfo, toJSON []byte) (map[string][]interface{}, error) {
	clientlog.Info("validating pipeline resources ", file.Name())
	time.Sleep(1 * time.Second)
	vsc := make(map[string][]interface{})
	convErr := yamlconv.Unmarshal(toJSON, &vsc)
	if convErr != nil {
		return nil, convErr
	}
	return vsc, nil
}

func convertYAMLToJSON(err error, readFile []byte) ([]byte, error) {
	toJSON, err := yamlconv.YAMLToJSON(readFile)
	if err != nil {
		clientlog.Error("Failed to convert to json")
		return nil, err
	}
	return toJSON, nil
}

func splitDataToPipelinesAndResourcesMap(vsc map[string][]interface{}, marErr error, err error, ymlType string) (map[string]string, error, bool) {
	resMap := make(map[string]string)
	var data []byte
	if v, ok := vsc["resources"]; ok {
		clientlog.Info("resources found preparing to validate")
		data, marErr = json.Marshal(v)
		if marErr != nil {
			clientlog.Error("failed to marshal to json")
			return nil, err, true
		}

		ymlType = "resources"
		resMap[ymlType] = string(data)

	}
	if vp, ok := vsc["pipelines"]; ok {
		clientlog.Info("pipelines found preparing to validate")
		data, marErr = json.Marshal(vp)
		if marErr != nil {
			clientlog.Error("failed to marshal to json")
			return nil, err, true
		}
		ymlType = "pipelines"
		fmt.Println(string(data))
		resMap[ymlType] = string(data)
	}
	//fmt.Printf("%+v \n", resMap)
	return resMap, nil, false
}

func getPayloadToValidatePipelineResource(resMap map[string]string) (*bytes.Buffer, error) {
	payload := getPayloadBasedOnYmlType(resMap)
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(payload)
	if err != nil {
		clientlog.Error("Failed to read stream to send payload to trigger pipelines")
		return nil, err
	}
	return buf, err
}

func getPayloadBasedOnYmlType(m map[string]string) *strings.Reader {
	var resReader, pipReader, valReader *strings.Reader
	for ymlType, _ := range m {
		if ymlType == "resources" {
			resReader = strings.NewReader(`{"fileName":"` + ymlType + `.yml","content":` + m[ymlType] + `,"ymlType":"` + ymlType + `"}`)
		} else if ymlType == "pipelines" {
			//fmt.Printf("data : %+v \n", m[ymlType])
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
	//fmt.Printf("exact data : %+v \n", valReader)
	return valReader
}
