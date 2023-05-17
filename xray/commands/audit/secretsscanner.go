package audit

import (
	"errors"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"gopkg.in/yaml.v2"
	"os"
	"path/filepath"
)

const (
	secretsScanCommand   = "sec"
	secretsScannersNames = "tokens, entropy"
	secretsScannerType   = "secrets-scan"
)

type Secret struct {
	Severity   string
	File       string
	LineColumn string
	Type       string
	Text       string
}

type SecretScanManager struct {
	secretsScannerResults []Secret
	configFileName        string
	resultsFileName       string
	analyzerManager       utils.AnalyzerManagerInterface
	serverDetails         *config.ServerDetails
	projectRootPath       string
}

func getSecretsScanResults(serverDetails *config.ServerDetails, analyzerManager utils.AnalyzerManagerInterface) ([]Secret, bool, error) {
	secretScanManager, err := NewSecretsScanManager(serverDetails, analyzerManager)
	if err != nil {
		log.Info("failed to run secrets scan: " + err.Error())
		return nil, false, err
	}
	err = secretScanManager.Run()
	if err != nil {
		if utils.IsNotEntitledError(err) || utils.IsUnsupportedCommandError(err) {
			return nil, false, nil
		}
		log.Info("failed to run secrets scan: " + err.Error())
		return nil, true, err
	}
	return secretScanManager.secretsScannerResults, true, nil
}

func NewSecretsScanManager(serverDetails *config.ServerDetails, analyzerManager utils.AnalyzerManagerInterface) (*SecretScanManager, error) {
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

func (s *SecretScanManager) Run() error {
	var err error
	defer func() {
		if deleteJasProcessFiles(s.configFileName, s.resultsFileName) != nil {
			e := deleteJasProcessFiles(s.configFileName, s.resultsFileName)
			if err == nil {
				err = e
			}
		}
	}()
	if err = s.createConfigFile(); err != nil {
		return err
	}
	if err = s.runAnalyzerManager(); err != nil {
		return err
	}
	if err = s.parseResults(); err != nil {
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
	s.projectRootPath = currentDir
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
	err := utils.SetAnalyzerManagerEnvVariables(s.serverDetails)
	if err != nil {
		return err
	}
	err = s.analyzerManager.Exec(s.configFileName, secretsScanCommand)
	return err
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
			Severity:   utils.GetResultSeverity(secret),
			File:       utils.GetResultFileName(secret),
			LineColumn: utils.GetResultLocationInFile(secret),
			Text:       s.getHiddenSecret(*secret.Locations[0].PhysicalLocation.Region.Snippet.Text),
			Type:       *secret.RuleID,
		}
		finalSecretsList = append(finalSecretsList, newSecret)
	}
	s.secretsScannerResults = finalSecretsList
	return nil
}

func (s *SecretScanManager) getHiddenSecret(secret string) string {
	if secret == "" {
		return ""
	}
	hiddenSecret := ""
	if len(secret) <= 3 {
		for i := 0; i < len(secret); i++ {
			hiddenSecret += "*"
		}
	} else { // show first 7 digits
		i := 0
		for i < 3 {
			hiddenSecret += string(secret[i])
			i++
		}
		for i < 15 {
			hiddenSecret += "*"
			i++
		}
	}
	return hiddenSecret
}
