package curation

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jfrog/gofrog/parallel"
	rtUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	audit "github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/generic"
	cmdUtils "github.com/jfrog/jfrog-cli-core/v2/xray/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/auth"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

const (
	// The "blocked" represents the unapproved status that can be returned by the Curation Service for dependencies..
	blocked                = "blocked"
	BlockingReasonPolicy   = "Policy violations"
	BlockingReasonNotFound = "Package pending update"

	totalConcurrentRequests = 10
	directRelation          = "direct"
	indirectRelation        = "indirect"

	BlockMessageKey  = "jfrog packages curation"
	NotBeingFoundKey = "not being found"

	extractPoliciesRegexTemplate = "({.*?})"
)

var supportedTech = map[coreutils.Technology]struct{}{
	coreutils.Npm: {},
}

type ErrorsResp struct {
	Errors []ErrorResp `json:"errors"`
}

type ErrorResp struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

type PackageStatus struct {
	Action            string   `json:"action"`
	ParentName        string   `json:"direct_dependency_package_name"`
	ParentVersion     string   `json:"direct_dependency_package_version"`
	BlockedPackageUrl string   `json:"blocked_package_url"`
	PackageName       string   `json:"blocked_package_name"`
	PackageVersion    string   `json:"blocked_package_version"`
	BlockingReason    string   `json:"blocking_reason"`
	DepRelation       string   `json:"dependency_relation"`
	PkgType           string   `json:"type"`
	Policy            []Policy `json:"policies"`
}

type Policy struct {
	Policy    string `json:"policy"`
	Condition string `json:"condition"`
}

type PackageStatusTable struct {
	Status            string        `col-name:"Action"`
	ParentName        string        `col-name:"Direct Dependency\nPackage Name"`
	ParentVersion     string        `col-name:"Direct Dependency\nPackage Version"`
	BlockedPackageUrl string        `col-name:"Blocked Package URL"`
	PackageName       string        `col-name:"Blocked Package\nName"`
	PackageVersion    string        `col-name:"Blocked Package\nVersion"`
	BlockingReason    string        `col-name:"Blocking Reason"`
	PkgType           string        `col-name:"Package Type"`
	Policy            []policyTable `embed-table:"true"`
}

type policyTable struct {
	Policy    string `col-name:"Violated Policy\nName"`
	Condition string `col-name:"Violated Condition\nName"`
}

type treeAnalyzer struct {
	rtManager            artifactory.ArtifactoryServicesManager
	extractPoliciesRegex *regexp.Regexp
	rtAuth               auth.ServiceDetails
	httpClientDetails    httputils.HttpClientDetails
	url                  string
	repo                 string
	tech                 coreutils.Technology
	parallelRequests     int
}

type CurationAuditCommand struct {
	PackageManagerConfig *rtUtils.RepositoryConfig
	extractPoliciesRegex *regexp.Regexp
	workingDirs          []string
	OriginPath           string
	parallelRequests     int
	*utils.GraphBasicParams
}

func NewCurationAuditCommand() *CurationAuditCommand {
	return &CurationAuditCommand{
		extractPoliciesRegex: regexp.MustCompile(extractPoliciesRegexTemplate),
		GraphBasicParams:     &utils.GraphBasicParams{},
	}
}

func (ca *CurationAuditCommand) setPackageManagerConfig(pkgMangerConfig *rtUtils.RepositoryConfig) *CurationAuditCommand {
	ca.PackageManagerConfig = pkgMangerConfig
	return ca
}

func (ca *CurationAuditCommand) SetWorkingDirs(dirs []string) *CurationAuditCommand {
	ca.workingDirs = dirs
	return ca
}

func (ca *CurationAuditCommand) SetParallelRequests(threads int) *CurationAuditCommand {
	ca.parallelRequests = threads
	return ca
}

func (ca *CurationAuditCommand) Run() (err error) {
	rootDir, err := os.Getwd()
	if err != nil {
		return errorutils.CheckError(err)
	}
	if len(ca.workingDirs) > 0 {
		defer func() {
			err = errorutils.CheckError(os.Chdir(rootDir))
		}()
	} else {
		ca.workingDirs = append(ca.workingDirs, rootDir)
	}
	results := map[string][]*PackageStatus{}
	for _, workDir := range ca.workingDirs {
		absWd, err := filepath.Abs(workDir)
		if err != nil {
			return errorutils.CheckError(err)
		}
		log.Info("Running curation audit on project:", absWd)
		if absWd != rootDir {
			if err = os.Chdir(absWd); err != nil {
				return errorutils.CheckError(err)
			}
		}
		// If error returned, continue to print results(if any), and return error at the end.
		err = ca.doCurateAudit(results)
	}
	if ca.Progress != nil {
		if err = ca.Progress.Quit(); err != nil {
			return err
		}
	}
	for projectPath, packagesStatus := range results {
		if err = printResult(ca.OutputFormat, projectPath, packagesStatus); err != nil {
			return err
		}
	}
	return
}

func (ca *CurationAuditCommand) doCurateAudit(results map[string][]*PackageStatus) error {
	techs, err := cmdUtils.DetectedTechnologies()
	if err != nil {
		return err
	}
	for _, tech := range techs {
		if _, ok := supportedTech[coreutils.Technology(tech)]; !ok {
			log.Info(fmt.Sprintf("It looks like this project uses '%s' to download its dependencies. "+
				"This package manager however isn't supported by this command.", tech))
			continue
		}
		if err = ca.auditTree(coreutils.Technology(tech), results); err != nil {
			return err
		}
	}
	return nil
}

func (ca *CurationAuditCommand) auditTree(tech coreutils.Technology, results map[string][]*PackageStatus) error {
	flattenGraph, err := audit.GetTechDependencyTree(ca.GraphBasicParams, tech)
	if err != nil {
		return err
	}
	// Validate the graph isn't empty.
	if len(ca.DependencyTrees) == 0 {
		return errorutils.CheckErrorf("found no dependencies for the audited project using '%v' as the package manager", tech.ToString())
	}
	if err = ca.SetRepo(tech); err != nil {
		return err
	}
	// Resolve the dependencies of the project.
	serverDetails, err := ca.PackageManagerConfig.ServerDetails()
	if err != nil {
		return err
	}
	rtManager, err := rtUtils.CreateServiceManager(serverDetails, 2, 0, false)
	if err != nil {
		return err
	}
	rtAuth, err := serverDetails.CreateArtAuthConfig()
	if err != nil {
		return err
	}
	_, projectName, projectVersion := getUrlNameAndVersionByTech(tech, ca.DependencyTrees[0].Id, "", "")
	if ca.Progress != nil {
		ca.Progress.SetHeadlineMsg(fmt.Sprintf("Fetch curation status for %s graph with %v nodes project name: %s:%s", tech.ToFormal(), len(flattenGraph[0].Nodes)-1, projectName, projectVersion))
	}
	if ca.parallelRequests == 0 {
		ca.parallelRequests = cmdUtils.TotalConcurrentRequests
	}
	var packagesStatus []*PackageStatus
	analyzer := treeAnalyzer{
		rtManager:            rtManager,
		extractPoliciesRegex: ca.extractPoliciesRegex,
		rtAuth:               rtAuth,
		httpClientDetails:    rtAuth.CreateHttpClientDetails(),
		url:                  rtAuth.GetUrl(),
		repo:                 ca.PackageManagerConfig.TargetRepo(),
		tech:                 tech,
		parallelRequests:     ca.parallelRequests,
	}
	packagesStatusMap := sync.Map{}
	// Root node id represents the project name and shouldn't be validated with curation
	rootNodeId := ca.DependencyTrees[0].Id
	// Fetch status for each node from a flatten graph which, has no duplicate nodes.
	err = analyzer.getNodesStatusInParallel(flattenGraph[0], &packagesStatusMap, rootNodeId)
	analyzer.fillGraphRelations(ca.DependencyTrees[0], &packagesStatusMap,
		&packagesStatus, "", "", true)
	sort.Slice(packagesStatus, func(i, j int) bool {
		return packagesStatus[i].ParentName < packagesStatus[j].ParentName
	})
	results[fmt.Sprintf("%s:%s", projectName, projectVersion)] = packagesStatus
	return err
}

func printResult(format utils.OutputFormat, projectPath string, packagesStatus []*PackageStatus) error {
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
		err := coreutils.PrintTable(pkgStatusTable, "Curation", "Found 0 blocked packages", true)
		if err != nil {
			return err
		}
	}
	log.Output("\n")
	return nil
}

func convertToTableStruct(packagesStatus []*PackageStatus) []PackageStatusTable {
	var pkgStatusTable []PackageStatusTable
	for _, pkgStatus := range packagesStatus {
		pkgTable := PackageStatusTable{
			Status:            pkgStatus.Action,
			ParentName:        pkgStatus.ParentName,
			ParentVersion:     pkgStatus.ParentVersion,
			BlockedPackageUrl: pkgStatus.BlockedPackageUrl,
			PackageName:       pkgStatus.PackageName,
			PackageVersion:    pkgStatus.PackageVersion,
			BlockingReason:    pkgStatus.BlockingReason,
			PkgType:           pkgStatus.PkgType,
		}
		var policiesCondTable []policyTable
		for _, policyCond := range pkgStatus.Policy {
			policiesCondTable = append(policiesCondTable, policyTable(policyCond))
		}
		pkgTable.Policy = policiesCondTable
		pkgStatusTable = append(pkgStatusTable, pkgTable)
	}
	return pkgStatusTable
}

func (ca *CurationAuditCommand) CommandName() string {
	return "curation"
}

func (ca *CurationAuditCommand) SetRepo(tech coreutils.Technology) error {
	switch tech {
	case coreutils.Npm:
		configFilePath, exists, err := rtUtils.GetProjectConfFilePath(rtUtils.Npm)
		if err != nil {
			return err
		}
		if !exists {
			return errorutils.CheckError(errors.New("no config file was found! Before running the npm command on a " +
				"project for the first time, the project should be configured using the jf npmc command"))
		}
		vConfig, err := rtUtils.ReadConfigFile(configFilePath, rtUtils.YAML)
		if err != nil {
			return err
		}
		resolverParams, err := rtUtils.GetRepoConfigByPrefix(configFilePath, rtUtils.ProjectConfigResolverPrefix, vConfig)
		if err != nil {
			return err
		}
		ca.setPackageManagerConfig(resolverParams)
	default:
		return errorutils.CheckErrorf("It looks like this project uses '%s' to download its dependencies. "+
			"This package manager however isn't supported by this command", tech.ToString())
	}
	return nil
}

func (nc *treeAnalyzer) fillGraphRelations(node *xrayUtils.GraphNode, preProcessMap *sync.Map,
	packagesStatus *[]*PackageStatus, parent, parentVersion string, isRoot bool) {
	for _, child := range node.Nodes {
		packageUrl, name, version := getUrlNameAndVersionByTech(nc.tech, child.Id, nc.url, nc.repo)
		if isRoot {
			parent = name
			parentVersion = version
		}
		if pkgStatus, exist := preProcessMap.Load(packageUrl); exist {
			relation := indirectRelation
			if isRoot {
				relation = directRelation
			}
			pkgStatusCast, isPkgStatus := pkgStatus.(*PackageStatus)
			if isPkgStatus {
				pkgStatusClone := *pkgStatusCast
				pkgStatusClone.DepRelation = relation
				pkgStatusClone.ParentName = parent
				pkgStatusClone.ParentVersion = parentVersion
				*packagesStatus = append(*packagesStatus, &pkgStatusClone)
			}
		}
		nc.fillGraphRelations(child, preProcessMap, packagesStatus, parent, parentVersion, false)
	}
}
func (nc *treeAnalyzer) getNodesStatusInParallel(graph *xrayUtils.GraphNode, p *sync.Map, rootNodeId string) error {
	var multiErrors error
	consumerProducer := parallel.NewBounedRunner(nc.parallelRequests, false)
	errorsQueue := clientutils.NewErrorsQueue(1)
	go func() {
		defer consumerProducer.Done()
		for _, node := range graph.Nodes {
			if node.Id == rootNodeId {
				continue
			}
			getTask := func(node xrayUtils.GraphNode) func(threadId int) error {
				return func(threadId int) (err error) {
					return nc.fetchNodeStatus(node, p)
				}
			}
			if _, err := consumerProducer.AddTaskWithError(getTask(*node), errorsQueue.AddError); err != nil {
				multiErrors = errors.Join(err, multiErrors)
			}
		}
	}()
	consumerProducer.Run()
	if err := errorsQueue.GetError(); err != nil {
		multiErrors = errors.Join(err, multiErrors)
	}
	return multiErrors
}

func (nc *treeAnalyzer) fetchNodeStatus(node xrayUtils.GraphNode, p *sync.Map) error {
	packageUrl, name, version := getUrlNameAndVersionByTech(nc.tech, node.Id, nc.url, nc.repo)
	resp, _, err := nc.rtManager.Client().SendHead(packageUrl, &nc.httpClientDetails)
	if err != nil {
		if resp != nil && resp.StatusCode >= 400 {
			return fmt.Errorf("failed sending HEAD to %s for package %s. Status-code: %v", packageUrl, node.Id, resp.StatusCode)
		}
		if resp == nil || resp.StatusCode != http.StatusForbidden {
			return err
		}

	}
	if resp != nil && resp.StatusCode >= 400 && resp.StatusCode != http.StatusForbidden {
		return fmt.Errorf("failed sending HEAD to %s for package %s. Status-code: %v", packageUrl, node.Id, resp.StatusCode)
	}
	if resp.StatusCode == http.StatusForbidden {
		pkStatus, err := nc.getBlockedPackageDetails(packageUrl, name, version)
		if err != nil {
			return err
		}
		if pkStatus != nil {
			p.Store(pkStatus.BlockedPackageUrl, pkStatus)
		}
	}
	return nil
}

// We try to collect curation details from GET response after HEAD request got forbidden status code.
func (nc *treeAnalyzer) getBlockedPackageDetails(packageUrl string, name string, version string) (*PackageStatus, error) {
	getResp, respBody, _, err := nc.rtManager.Client().SendGet(packageUrl, true, &nc.httpClientDetails)
	if err != nil {
		if getResp == nil {
			return nil, err
		}
		if getResp.StatusCode != http.StatusForbidden {
			return nil, fmt.Errorf("failed sending HEAD request to %s for package '%s:%s'. "+
				"Status code: %v. Cause: %v", packageUrl, name, version, getResp.StatusCode, err)
		}
	}
	if getResp.StatusCode == http.StatusForbidden {
		respError := &ErrorsResp{}
		if err := json.Unmarshal(respBody, respError); err != nil {
			return nil, errorutils.CheckError(err)
		}
		if len(respError.Errors) == 0 {
			return nil, errorutils.CheckErrorf("received 403 for unknown reason, no curation status will be presented for this package. "+
				"package name: %s, version: %s, download url: %s ", name, version, packageUrl)
		}
		// if the error message contains the curation string key, then we can be sure it got blocked by Curation service.
		if strings.Contains(strings.ToLower(respError.Errors[0].Message), BlockMessageKey) {
			blockingReason := BlockingReasonPolicy
			if strings.Contains(strings.ToLower(respError.Errors[0].Message), NotBeingFoundKey) {
				blockingReason = BlockingReasonNotFound
			}
			policies := nc.extractPoliciesFromMsg(respError)
			return &PackageStatus{
				PackageName:       name,
				PackageVersion:    version,
				BlockedPackageUrl: packageUrl,
				Action:            blocked,
				Policy:            policies,
				BlockingReason:    blockingReason,
				PkgType:           string(nc.tech),
			}, nil
		}
	}
	return nil, nil
}

// Return policies and conditions names from the FORBIDDEN HTTP error message.
// Message structure: Package %s:%s download was blocked by JFrog Packages Curation service due to the following policies violated {%s, %s},{%s, %s}.
func (nc *treeAnalyzer) extractPoliciesFromMsg(respError *ErrorsResp) []Policy {
	var policies []Policy
	msg := respError.Errors[0].Message
	allMatches := nc.extractPoliciesRegex.FindAllString(msg, -1)
	for _, match := range allMatches {
		match = strings.TrimSuffix(strings.TrimPrefix(match, "{"), "}")
		polCond := strings.Split(match, ",")
		if len(polCond) == 2 {
			pol := polCond[0]
			cond := polCond[1]
			policies = append(policies, Policy{Policy: strings.TrimSpace(pol), Condition: strings.TrimSpace(cond)})
		}
	}
	return policies
}

func getUrlNameAndVersionByTech(tech coreutils.Technology, nodeId, artiUrl, repo string) (downloadUrl string, name string, version string) {
	if tech == coreutils.Npm {
		return getNameScopeAndVersion(nodeId, artiUrl, repo, coreutils.Npm.ToString())
	}
	return
}

// The graph holds, for each node, the component ID (xray representation)
// from which we extract the package name, version, and construct the Artifactory download URL.
func getNameScopeAndVersion(id, artiUrl, repo, tech string) (downloadUrl, name, version string) {
	id = strings.TrimPrefix(id, tech+"://")

	nameVersion := strings.Split(id, ":")
	name = nameVersion[0]
	if len(nameVersion) > 1 {
		version = nameVersion[1]
	}
	scopeSplit := strings.Split(name, "/")
	var scope string
	if len(scopeSplit) > 1 {
		scope = scopeSplit[0]
		name = scopeSplit[1]
	}
	return buildNpmDownloadUrl(artiUrl, repo, name, scope, version), name, version
}

func buildNpmDownloadUrl(url, repo, name, scope, version string) string {
	var packageUrl string
	if scope != "" {
		packageUrl = fmt.Sprintf("%s/api/npm/%s/%s/%s/-/%s-%s.tgz", strings.TrimSuffix(url, "/"), repo, scope, name, name, version)
	} else {
		packageUrl = fmt.Sprintf("%s/api/npm/%s/%s/-/%s-%s.tgz", strings.TrimSuffix(url, "/"), repo, name, name, version)
	}
	return packageUrl
}
