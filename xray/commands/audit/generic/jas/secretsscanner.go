package jas

import (
	"errors"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"gopkg.in/yaml.v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	secretsScanCommand   = "sec"
	secretsScannersNames = "tokens, entropy"
	secretsScannerType   = "secrets-scan"
)

type Secret struct {
	File     string
	Location string
	Type     string
	Text     string
}

type SecretScanManager struct {
	secretsScannerResults []Secret
	configFileName        string
	resultsFileName       string
	analyzerManager       AnalyzerManager
	serverDetails         *config.ServerDetails
}

func getSecretsScanResults(serverDetails *config.ServerDetails, analyzerManager AnalyzerManager) ([]Secret, bool, error) {
	secretScanManager, err := NewsSecretsScanManager(serverDetails, analyzerManager)
	if err != nil {
		log.Info("failed to run secrets scan: " + err.Error())
		return nil, false, err
	}
	entitledForSecretsScan, err := secretScanManager.Run()
	if !entitledForSecretsScan {
		log.Info("got not entitled error from analyzer manager")
		return nil, false, nil
	}
	if err != nil {
		log.Info("failed to run secrets scan: " + err.Error())
		return nil, true, err
	}
	return secretScanManager.secretsScannerResults, true, nil
}

func NewsSecretsScanManager(serverDetails *config.ServerDetails, analyzerManager AnalyzerManager) (*SecretScanManager, error) {
	if serverDetails == nil {
		return nil, errors.New("cant get xray server details")
	}
	tempDir, err := fileutils.CreateTempDir()
	if err != nil {
		return nil, err
	}
	return &SecretScanManager{
		secretsScannerResults: []Secret{},
		configFileName:        filepath.Join(tempDir, "config.yaml"),
		resultsFileName:       filepath.Join(tempDir, "results.sarif"),
		analyzerManager:       analyzerManager,
		serverDetails:         serverDetails,
	}, nil
}

func (s *SecretScanManager) Run() (bool, error) {
	defer deleteJasScanProcessFiles(s.configFileName, s.resultsFileName)
	if err := s.createConfigFile(); err != nil {
		return true, err
	}
	if entitledForSecretsScan, err := s.runAnalyzerManager(); err != nil {
		return entitledForSecretsScan, err
	}
	if err := s.parseResults(); err != nil {
		return true, err
	}
	return true, nil
}

type secretsScanConfig struct {
	Scans []secretsScanConfiguration `yaml:"scans"`
}

type secretsScanConfiguration struct {
	Roots    []string `yaml:"roots"`
	Output   string   `yaml:"output"`
	Type     string   `yaml:"type"`
	Scanners string   `yaml:"scanners"`
}

func (s *SecretScanManager) createConfigFile() error {
	currentDir, err := coreutils.GetWorkingDirectory()
	if err != nil {
		return err
	}
	configFileContent := secretsScanConfig{
		Scans: []secretsScanConfiguration{
			{
				Roots:    []string{currentDir},
				Output:   filepath.Join(currentDir, s.resultsFileName),
				Type:     secretsScannerType,
				Scanners: secretsScannersNames,
			},
		},
	}
	yamlData, err := yaml.Marshal(&configFileContent)
	if err != nil {
		return err
	}
	err = os.WriteFile(s.configFileName, yamlData, 0644)
	if err != nil {
		return err
	}
	return nil
}

func (s *SecretScanManager) runAnalyzerManager() (bool, error) {
	err := setAnalyzerManagerEnvVariables(s.serverDetails)
	if err != nil {
		return true, err
	}
	currentDir, err := coreutils.GetWorkingDirectory()
	if err != nil {
		return true, err
	}
	err = s.analyzerManager.RunAnalyzerManager(filepath.Join(currentDir, s.configFileName), secretsScanCommand)
	if err != nil {
		if notEntitledError := isNotEntitledError(err); notEntitledError {
			return false, err
		}
	}
	return true, err
}

func (s *SecretScanManager) parseResults() error {
	report, err := sarif.Open(s.resultsFileName)
	if err != nil {
		return err
	}
	var secretsResults []*sarif.Result
	if len(report.Runs) > 0 {
		secretsResults = report.Runs[0].Results
	}

	finalSecretsList := []Secret{}

	for _, secret := range secretsResults {
		newSecret := Secret{
			File:     getSecretFileName(secret),
			Location: getSecretLocation(secret),
			Text:     getHiddenSecret(*secret.Locations[0].PhysicalLocation.Region.Snippet.Text),
			Type:     *secret.RuleID,
		}
		finalSecretsList = append(finalSecretsList, newSecret)
	}
	s.secretsScannerResults = finalSecretsList
	return nil
}

func getSecretFileName(secret *sarif.Result) string {
	file := secret.Locations[0].PhysicalLocation.ArtifactLocation.URI
	if file == nil {
		return ""
	}
	splitFileArray := strings.Split(*file, "///")
	if len(splitFileArray) > 1 {
		return splitFileArray[1]
	}
	return splitFileArray[0]
}

func getSecretLocation(secret *sarif.Result) string {
	startLine := strconv.Itoa(*secret.Locations[0].PhysicalLocation.Region.StartLine)
	startColumn := strconv.Itoa(*secret.Locations[0].PhysicalLocation.Region.StartColumn)
	if startLine != "" && startColumn != "" {
		return startLine + ":" + startColumn
	} else if startLine == "" && startColumn != "" {
		return "startLine:" + startColumn
	} else if startLine != "" && startColumn == "" {
		return startLine + ":startColumn"
	}
	return ""
}

func getHiddenSecret(secret string) string {
	if secret == "" {
		return ""
	}
	hiddenSecret := ""
	if len(secret) <= 10 { // short secret - hide all digits
		for i := 0; i < len(secret); i++ {
			hiddenSecret += "*"
		}
	} else { // show first 7 digits
		i := 0
		for i < 7 {
			hiddenSecret += string(secret[i])
			i++
		}
		for i < 30 {
			hiddenSecret += "*"
			i++
		}
	}
	return hiddenSecret
}
