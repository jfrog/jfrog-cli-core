package curation

import (
	"errors"
	"fmt"
	artifactory_utils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	audit "github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/generic"
	cmdUtils "github.com/jfrog/jfrog-cli-core/v2/xray/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"path/filepath"
)

const (
	Blocked = "blocked"
)

var supportedTech = map[coreutils.Technology]struct{}{
	coreutils.Npm: {},
}

type PackageStatus struct {
	Action         string   `json:"action"`
	PackageName    string   `json:"blocked_package_name"`
	PackageVersion string   `json:"blocked_package_version"`
	ParentName     string   `json:"direct_dependency_package_name"`
	ParentVersion  string   `json:"direct_dependency_package_version"`
	DepRelation    string   `json:"dependency_relation"`
	PkgType        string   `json:"type"`
	Policy         []Policy `json:"policies"`
	Resolved       string   `json:"resolved"`
}

type Policy struct {
	Policy    string `json:"policy"`
	Condition string `json:"condition"`
}

type PackageStatusTableStruct struct {
	Status         string              `col-name:"Action"`
	ParentName     string              `col-name:"Direct Dependency\nPackage Name"`
	ParentVersion  string              `col-name:"Direct Dependency\nPackage Version"`
	PackageName    string              `col-name:"Blocked Package\nName"`
	PackageVersion string              `col-name:"Blocked Package\nVersion"`
	PkgType        string              `col-name:"Package Type"`
	Policy         []policyTableStruct `embed-table:"true"`
}

type policyTableStruct struct {
	Policy    string `col-name:"Violated Policy\nName"`
	Condition string `col-name:"Violated Condition\nName"`
}

type Command struct {
	PackageManagerConfig *artifactory_utils.RepositoryConfig
	workingDirs          []string
	OriginPath           string
	*cmdUtils.GraphBasicParams
}

func NewCurationCommand() *Command {
	return &Command{}
}

func (ss *Command) SetPackageManagerConfig(pkgMangerConfig *artifactory_utils.RepositoryConfig) *Command {
	ss.PackageManagerConfig = pkgMangerConfig
	return ss
}

func (ss *Command) SetWorkingDirs(dirs []string) *Command {
	ss.workingDirs = dirs
	return ss
}

func (ss *Command) Run() (err error) {
	rootDir, err := os.Getwd()
	if err != nil {
		return errorutils.CheckError(err)
	}
	if len(ss.workingDirs) > 0 {
		defer func() {
			e := os.Chdir(rootDir)
			if err == nil {
				err = e
			}
		}()
	} else {
		ss.workingDirs = append(ss.workingDirs, rootDir)
	}
	results := map[string][]PackageStatus{}
	for _, workDir := range ss.workingDirs {
		absWd, err := filepath.Abs(workDir)
		if err != nil {
			return errorutils.CheckError(err)
		}
		log.Info("curation project:", absWd)
		if absWd != rootDir {
			err = os.Chdir(absWd)
			if err != nil {
				return err
			}
		}
		err = ss.curateProject(results)
		if err != nil {
			return err
		}
	}
	if ss.Progress != nil {
		err = ss.Progress.Quit()
		if err != nil {
			return err
		}
	}
	for projectPath, packagesStatus := range results {
		err = printResult(ss.OutputFormat, projectPath, packagesStatus)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ss *Command) curateProject(results map[string][]PackageStatus) error {
	techs, err := cmdUtils.DetectedTechnologies()
	if err != nil {
		return err
	}
	for _, tech := range techs {
		if _, ok := supportedTech[coreutils.Technology(tech)]; !ok {
			log.Info(fmt.Sprintf("packge type %s is not supported by curation cli", tech))
			continue
		}
		err = ss.curateTree(coreutils.Technology(tech), results)
		if err != nil {
			return err
		}
	}
	return err
}

func (ss *Command) curateTree(tech coreutils.Technology, results map[string][]PackageStatus) error {
	_, err := audit.GetTechDependencyTree(ss.GraphBasicParams, tech)
	if err != nil {
		return err
	}
	// we check the graph filled
	if len(ss.DependencyTrees) == 0 {
		return fmt.Errorf("failed to get graph for package type %v", tech)
	}
	err = ss.SetRegistryByTech(tech)
	if err != nil {
		return err
	}
	// resolve packages from the package manager configured for the project.
	serverDetails, err := ss.PackageManagerConfig.ServerDetails()
	if err != nil {
		return err
	}
	artiManager, err := artifactory_utils.CreateServiceManager(serverDetails, 2, 0, false)
	if err != nil {
		return err
	}
	artAuth, err := serverDetails.CreateArtAuthConfig()
	if err != nil {
		return err
	}
	artiHttpClientDetails := artAuth.CreateHttpClientDetails()
	_, projectName, projectVersion := getUrlNameAndVersionByTech(tech, ss.DependencyTrees[0].Id, "", "")
	if ss.Progress != nil {
		ss.Progress.SetHeadlineMsg(fmt.Sprintf("Fetch curation block status for %s graph, project %s:%s", tech.ToFormal(), projectName, projectVersion))
	}
	var packagesStatus []PackageStatus
	analyzer := treeAnalyzer{
		artiManager:       artiManager,
		artAuth:           artAuth,
		httpClientDetails: artiHttpClientDetails,
		url:               artAuth.GetUrl(),
		repo:              ss.PackageManagerConfig.TargetRepo(),
		tech:              tech,
	}
	if err := analyzer.recursiveNodeCuration(ss.DependencyTrees[0], &packagesStatus, "", "", true); err != nil {
		return err
	}
	results[fmt.Sprintf("%s:%s", projectName, projectVersion)] = packagesStatus
	return nil
}

func printResult(format utils.OutputFormat, projectPath string, packagesStatus []PackageStatus) error {
	if format == "" {
		format = utils.Table
	}
	log.Output(fmt.Sprintf("Found %v blocked packges for project %s", len(packagesStatus), projectPath))
	switch format {
	case utils.Json:
		if len(packagesStatus) > 0 {
			err := utils.PrintJson(packagesStatus)
			if err != nil {
				return err
			}
		}
	case utils.Table:
		pkgStatusTable := convertToTableStruct(packagesStatus)
		err := coreutils.PrintTable(pkgStatusTable, "Curation", "Found 0 blocked packages", false)
		if err != nil {
			return err
		}
	}
	log.Output("\n")
	return nil
}

func convertToTableStruct(packagesStatus []PackageStatus) []PackageStatusTableStruct {
	var pkgStatusTable []PackageStatusTableStruct
	for _, pkgStatus := range packagesStatus {
		pkgTable := PackageStatusTableStruct{
			Status:         pkgStatus.Action,
			ParentName:     pkgStatus.ParentName,
			ParentVersion:  pkgStatus.ParentVersion,
			PackageName:    pkgStatus.PackageName,
			PackageVersion: pkgStatus.PackageVersion,
			PkgType:        pkgStatus.PkgType,
		}
		var policiesCondTable []policyTableStruct
		for _, policyCond := range pkgStatus.Policy {
			policiesCondTable = append(policiesCondTable, policyTableStruct(policyCond))
		}
		pkgTable.Policy = policiesCondTable
		pkgStatusTable = append(pkgStatusTable, pkgTable)
	}
	return pkgStatusTable
}

func (ss *Command) CommandName() string {
	return "curation"
}

func (ss *Command) SetRegistryByTech(tech coreutils.Technology) error {
	switch tech {
	case coreutils.Npm:
		configFilePath, exists, err := artifactory_utils.GetProjectConfFilePath(artifactory_utils.Npm)
		if err != nil {
			return err
		}
		if !exists {
			return errorutils.CheckError(errors.New("no config file was found! Before running the npm command on a " +
				"project for the first time, the project should be configured using the npm-config command"))
		}
		vConfig, err := artifactory_utils.ReadConfigFile(configFilePath, artifactory_utils.YAML)
		if err != nil {
			return err
		}
		resolverParams, err := artifactory_utils.GetRepoConfigByPrefix(configFilePath, artifactory_utils.ProjectConfigResolverPrefix, vConfig)
		if err != nil {
			return err
		}
		ss.SetPackageManagerConfig(resolverParams)
	default:
		return fmt.Errorf("package type %s not supported", tech.ToString())
	}
	return nil
}
