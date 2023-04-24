package jas

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/owenrumney/go-sarif/sarif"
	"gopkg.in/yaml.v2"
	"os"
	"path/filepath"
	"strconv"
)

const (
	secretsScanCommand   = "sec"
	secretsScannersNames = "tokens, entropy"
	secretsScannerType   = "secrets-scan"
)

type Secret struct {
	File string
	Line string
	Type string
	Text string
}

type SecretScanManager struct {
	secretsScannerResults []Secret
	configFileName        string
	resultsFileName       string
	analyzerManager       AnalyzerManager
	serverDetails         *config.ServerDetails
}

func getSecretsScanResults(serverDetails *config.ServerDetails) ([]Secret, error) {
	secretScanManager, err := NewsSecretsScanManager(serverDetails)
	if err != nil {
		return nil, handleSecretsScanError(err, secretScanManager)
	}
	if !secretScanManager.analyzerManager.DoesAnalyzerManagerExecutableExist() {
		log.Info("analyzer manager doesnt exist, user is not entitled for jas")
		return nil, nil
	}
	err = secretScanManager.Run()
	if err != nil {
		return nil, handleSecretsScanError(err, secretScanManager)
	}
	return secretScanManager.secretsScannerResults, nil
}

func NewsSecretsScanManager(serverDetails *config.ServerDetails) (*SecretScanManager, error) {
	configFileName, err := generateRandomFileName()
	if err != nil {
		return nil, err
	}
	resultsFileName, err := generateRandomFileName()
	if err != nil {
		return nil, err
	}
	return &SecretScanManager{
		secretsScannerResults: []Secret{},
		configFileName:        configFileName + ".yaml",
		resultsFileName:       resultsFileName + ".sarif",
		analyzerManager:       analyzerManagerExecuter,
		serverDetails:         serverDetails,
	}, nil
}

func handleSecretsScanError(err error, scanManager *SecretScanManager) error {
	log.Info("failed to run secrets scan: " + err.Error())
	deleteFilesError := deleteJasScanProcessFiles(scanManager.configFileName,
		scanManager.resultsFileName)
	if deleteFilesError != nil {
		return deleteFilesError
	}
	return err
}

func (s *SecretScanManager) Run() error {
	if err := s.createConfigFile(); err != nil {
		return err
	}
	if err := s.runAnalyzerManager(); err != nil {
		return err
	}
	if err := s.parseResults(); err != nil {
		return err
	}
	if err := deleteJasScanProcessFiles(s.configFileName, s.resultsFileName); err != nil {
		return err
	}
	return nil
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

func (s *SecretScanManager) runAnalyzerManager() error {
	err := setAnalyzerManagerEnvVariables(s.serverDetails)
	if err != nil {
		return err
	}
	currentDir, err := coreutils.GetWorkingDirectory()
	if err != nil {
		return err
	}
	err = s.analyzerManager.RunAnalyzerManager(filepath.Join(currentDir, s.configFileName), secretsScanCommand)
	if err != nil {
		return err
	}
	return nil
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
			File: *secret.Locations[0].PhysicalLocation.ArtifactLocation.URI,
			Line: strconv.Itoa(*secret.Locations[0].PhysicalLocation.Region.StartLine),
			Text: partiallyHideSecret(*secret.Locations[0].PhysicalLocation.Region.Snippet.Text),
			Type: *secret.RuleID,
		}
		finalSecretsList = append(finalSecretsList, newSecret)
	}
	return nil
}

func partiallyHideSecret(secret string) string {
	if secret == "" {
		return ""
	}
	hiddenSecret := ""
	if len(secret) <= 10 { // short secret - hide all digits
		for i := 0; i < len(secret); i++ {
			hiddenSecret += "*"
		}
	} else { // hide last 7 digits
		i := 0
		for i < len(secret)-7 {
			hiddenSecret += string(secret[i])
			i++
		}
		for i < len(secret) {
			hiddenSecret += "*"
			i++
		}
	}
	return hiddenSecret
}
