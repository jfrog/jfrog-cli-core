package jas

//
//import (
//	"errors"
//	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
//	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
//	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
//	"github.com/jfrog/jfrog-client-go/utils/log"
//	"github.com/owenrumney/go-sarif/v2/sarif"
//	"gopkg.in/yaml.v2"
//	"os"
//	"path/filepath"
//	"strconv"
//	"strings"
//)
//
//const (
//	iacScanCommand = "iac"
//	iacScannerType = "iac-scan-modules"
//)
//
//type Iac struct { // TODO
//	Severity   string
//	File       string
//	LineColumn string
//	Type       string
//	Text       string
//}
//
//type IacScanManager struct {
//	iacScannerResults []Iac
//	configFileName    string
//	resultsFileName   string
//	analyzerManager   AnalyzerManager
//	serverDetails     *config.ServerDetails
//	projectRootPath   string
//}
//
//func getIacScanResults(serverDetails *config.ServerDetails, analyzerManager AnalyzerManager) ([]Iac, bool, error) {
//	iacScanManager, err := NewsIacScanManager(serverDetails, analyzerManager)
//	if err != nil {
//		log.Info("failed to run iac scan: " + err.Error())
//		return nil, false, err
//	}
//	err = iacScanManager.Run()
//	if err != nil {
//		if isNotEntitledError(err) || isUnsupportedCommandError(err) {
//			return nil, false, nil
//		}
//		log.Info("failed to run iac scan: " + err.Error())
//		return nil, true, err
//	}
//	return iacScanManager.iacScannerResults, true, nil
//}
//
//func NewsIacScanManager(serverDetails *config.ServerDetails, analyzerManager AnalyzerManager) (*IacScanManager, error) {
//	if serverDetails == nil {
//		return nil, errors.New("cant get xray server details")
//	}
//	tempDir, err := fileutils.CreateTempDir()
//	if err != nil {
//		return nil, err
//	}
//	return &IacScanManager{
//		iacScannerResults: []Iac{},
//		configFileName:    filepath.Join(tempDir, "config.yaml"),
//		resultsFileName:   filepath.Join(tempDir, "results.sarif"),
//		analyzerManager:   analyzerManager,
//		serverDetails:     serverDetails,
//	}, nil
//}
//
//func (iac *IacScanManager) Run() error {
//	defer deleteJasScanProcessFiles(iac.configFileName, iac.resultsFileName)
//	if err := iac.createConfigFile(); err != nil {
//		return err
//	}
//	if err := iac.runAnalyzerManager(); err != nil {
//		return err
//	}
//	if err := iac.parseResults(); err != nil {
//		return err
//	}
//	return nil
//}
//
//type iacScanConfig struct {
//	Scans []iacScanConfiguration `yaml:"scans"`
//}
//
//type iacScanConfiguration struct {
//	Roots  []string `yaml:"roots"`
//	Output string   `yaml:"output"`
//	Type   string   `yaml:"type"`
//}
//
//func (iac *IacScanManager) createConfigFile() error {
//	currentDir, err := coreutils.GetWorkingDirectory()
//	iac.projectRootPath = currentDir
//	if err != nil {
//		return err
//	}
//	configFileContent := iacScanConfig{
//		Scans: []iacScanConfiguration{
//			{
//				Roots:  []string{currentDir},
//				Output: filepath.Join(currentDir, iac.resultsFileName),
//				Type:   iacScannerType,
//			},
//		},
//	}
//	yamlData, err := yaml.Marshal(&configFileContent)
//	if err != nil {
//		return err
//	}
//	err = os.WriteFile(iac.configFileName, yamlData, 0644)
//	if err != nil {
//		return err
//	}
//	return nil
//}
//
//func (iac *IacScanManager) runAnalyzerManager() error {
//	err := setAnalyzerManagerEnvVariables(iac.serverDetails)
//	if err != nil {
//		return err
//	}
//	currentDir, err := coreutils.GetWorkingDirectory()
//	if err != nil {
//		return err
//	}
//	err = iac.analyzerManager.RunAnalyzerManager(filepath.Join(currentDir, iac.configFileName), secretsScanCommand)
//	return err
//}
//
//func (s *SecretScanManager) parseResults() error {
//	report, err := sarif.Open(s.resultsFileName)
//	if err != nil {
//		return err
//	}
//	var secretsResults []*sarif.Result
//	if len(report.Runs) > 0 {
//		secretsResults = report.Runs[0].Results
//	}
//
//	finalSecretsList := []Secret{}
//
//	for _, secret := range secretsResults {
//		newSecret := Secret{
//			Severity:   s.getSecretSeverity(secret),
//			File:       s.getSecretFileName(secret),
//			LineColumn: s.getSecretLocation(secret),
//			Text:       s.getHiddenSecret(*secret.Locations[0].PhysicalLocation.Region.Snippet.Text),
//			Type:       *secret.RuleID,
//		}
//		finalSecretsList = append(finalSecretsList, newSecret)
//	}
//	s.secretsScannerResults = finalSecretsList
//	return nil
//}
//
//func (s *SecretScanManager) getSecretFileName(secret *sarif.Result) string {
//	filePath := secret.Locations[0].PhysicalLocation.ArtifactLocation.URI
//	if filePath == nil {
//		return ""
//	}
//	return s.extractRelativePath(*filePath)
//}
//
//func (s *SecretScanManager) getSecretLocation(secret *sarif.Result) string {
//	startLine := strconv.Itoa(*secret.Locations[0].PhysicalLocation.Region.StartLine)
//	startColumn := strconv.Itoa(*secret.Locations[0].PhysicalLocation.Region.StartColumn)
//	if startLine != "" && startColumn != "" {
//		return startLine + ":" + startColumn
//	} else if startLine == "" && startColumn != "" {
//		return "startLine:" + startColumn
//	} else if startLine != "" && startColumn == "" {
//		return startLine + ":startColumn"
//	}
//	return ""
//}
//
//func (s *SecretScanManager) getHiddenSecret(secret string) string {
//	if secret == "" {
//		return ""
//	}
//	hiddenSecret := ""
//	if len(secret) <= 3 {
//		for i := 0; i < len(secret); i++ {
//			hiddenSecret += "*"
//		}
//	} else { // show first 7 digits
//		i := 0
//		for i < 3 {
//			hiddenSecret += string(secret[i])
//			i++
//		}
//		for i < 15 {
//			hiddenSecret += "*"
//			i++
//		}
//	}
//	return hiddenSecret
//}
//
//func (s *SecretScanManager) extractRelativePath(secretPath string) string {
//	filePrefix := "file://"
//	relativePath := strings.ReplaceAll(strings.ReplaceAll(secretPath, s.projectRootPath, ""), filePrefix, "")
//	return relativePath
//}
//
//func (s *SecretScanManager) getSecretSeverity(secret *sarif.Result) string {
//	if secret.Level != nil {
//		return *secret.Level
//	}
//	return "Medium" // Default value for severity
//
//}
