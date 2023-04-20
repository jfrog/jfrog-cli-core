package curation

import (
	"errors"
	"fmt"
	artifactory_utils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	audit "github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/generic"
	cmdUtils "github.com/jfrog/jfrog-cli-core/v2/xray/commands/utils"
	utils2 "github.com/jfrog/jfrog-cli-core/v2/xray/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
)

const (
	Blocked = "blocked"
)

var supportedTech = map[coreutils.Technology]struct{}{
	coreutils.Npm: {},
}

type PackageStatus struct {
	PackageName    string   `json:"package_name"`
	PackageVersion string   `json:"package_version"`
	Status         string   `json:"status"`
	Parent         string   `json:"parent"`
	DepRelation    string   `json:"dependency_relation"`
	PkgType        string   `json:"type"`
	Policy         []policy `json:"policies"`
	Resolved       string   `json:"resolved"`
}

type policy struct {
	Policy    string `json:"policy"`
	Condition string `json:"condition"`
}

type PackageStatusTableStruct struct {
	Status         string              `col-name:"Action"`
	Parent         string              `col-name:"Direct Dependency\nPackage Name And Version"`
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
	params               *audit.Params
	ignoreConfigFile     bool
	workingDir           string
	installFunc          func(tech string) error
	OriginPath           string
	*cmdUtils.GraphBasicParams
}

func NewCurationCommand() *Command {
	return &Command{}
}

func (ss *Command) SetPackageManagerConfig(npmConfig *artifactory_utils.RepositoryConfig) *Command {
	ss.PackageManagerConfig = npmConfig
	return ss
}

func (ss *Command) SetWorkingDirs(dir string) *Command {
	ss.workingDir = dir
	return ss
}

func (ss *Command) Run() (err error) {
	defer func() {
		if ss.OriginPath != "" {
			e := os.Chdir(ss.OriginPath)
			if err == nil {
				err = e
			}
		}
	}()
	techs, err := utils2.DetectedTechnologies()
	if err != nil {
		return err
	}
	var packagesStatus []PackageStatus
	for _, tech := range techs {
		if _, ok := supportedTech[coreutils.Technology(tech)]; !ok {
			log.Info(fmt.Sprintf("packge type %s is not supported by curation cli", tech))
			continue
		}
		packagesStatus, err = ss.curateTree(coreutils.Technology(tech))
		if err != nil {
			return err
		}
	}
	if ss.Progress != nil {
		err = ss.Progress.Quit()
		if err != nil {
			return
		}
	}
	err = printResult(ss.OutputFormat, packagesStatus)
	if err != nil {
		return err
	}
	return nil
}

func (ss *Command) curateTree(tech coreutils.Technology) ([]PackageStatus, error) {
	graph, err := audit.GetTechDependencyTree(ss.GraphBasicParams, tech)
	if err != nil {
		return nil, err
	}
	if len(graph) == 0 {
		return nil, errors.New(fmt.Sprintf("failed to get graph for package type %v", tech))
	}
	err = ss.SetRegistryByTech(tech)
	if err != nil {
		return nil, err
	}
	// resolve packages from the package manager configured for the project.
	serverDetails, err := ss.PackageManagerConfig.ServerDetails()
	if err != nil {
		return nil, err
	}
	artiManager, err := artifactory_utils.CreateServiceManager(serverDetails, 2, 0, false)
	if err != nil {
		return nil, err
	}
	artAuth, err := serverDetails.CreateArtAuthConfig()
	if err != nil {
		return nil, err
	}
	artiHttpClientDetails := artAuth.CreateHttpClientDetails()
	url := artAuth.GetUrl()
	repo := ss.PackageManagerConfig.TargetRepo()
	analyzer := treeAnalyzer{
		artiManager:       artiManager,
		artAuth:           artAuth,
		httpClientDetails: artiHttpClientDetails,
		url:               url,
		repo:              repo,
		tech:              tech,
	}

	var packagesStatus []PackageStatus
	if ss.Progress != nil {
		ss.Progress.SetHeadlineMsg(fmt.Sprintf("Fetch curation block status for %s graph", tech.ToFormal()))
	}
	if err := analyzer.recursiveNodeCuration(graph[0], &packagesStatus, "", true); err != nil {
		return nil, err
	}
	return packagesStatus, nil
}

func printResult(format utils.OutputFormat, packagesStatus []PackageStatus) error {
	if format == "" {
		format = utils.Table
	}
	switch format {
	case utils.Json:
		log.Output(fmt.Sprintf("Found %v blocked packges", len(packagesStatus)))
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

	return nil
}

func convertToTableStruct(packagesStatus []PackageStatus) []PackageStatusTableStruct {
	var pkgStatusTable []PackageStatusTableStruct
	for _, pkgStatus := range packagesStatus {
		pkgTable := PackageStatusTableStruct{
			Status:         pkgStatus.Status,
			Parent:         pkgStatus.Parent,
			PackageName:    pkgStatus.PackageName,
			PackageVersion: pkgStatus.PackageVersion,
			PkgType:        pkgStatus.PkgType,
		}
		var policiesCondTable []policyTableStruct
		for _, policyCond := range pkgStatus.Policy {
			policiesCondTable = append(policiesCondTable, policyTableStruct{
				Policy:    policyCond.Policy,
				Condition: policyCond.Condition,
			})
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
		return errors.New(fmt.Sprintf("package type %s not supported", tech.ToString()))
	}
	return nil
}
