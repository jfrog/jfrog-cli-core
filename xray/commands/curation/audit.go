package curation

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jfrog/gofrog/datastructures"
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

	errorTemplateHeadRequest = "failed sending HEAD request to %s for package '%s:%s'. Status-code: %v. Cause: %v"

	errorTemplateUnsupportedTech = "It looks like this project uses '%s' to download its dependencies. " +
		"This package manager however isn't supported by this command."
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
	BlockedPackageUrl string   `json:"blocked_package_url,omitempty"`
	PackageName       string   `json:"blocked_package_name"`
	PackageVersion    string   `json:"blocked_package_version"`
	BlockingReason    string   `json:"blocking_reason"`
	DepRelation       string   `json:"dependency_relation"`
	PkgType           string   `json:"type"`
	Policy            []Policy `json:"policies,omitempty"`
}

type Policy struct {
	Policy         string `json:"policy"`
	Condition      string `json:"condition"`
	Explanation    string `json:"explanation"`
	Recommendation string `json:"recommendation"`
}

type PackageStatusTable struct {
	ParentName     string `col-name:"Direct\nDependency\nPackage\nName" auto-merge:"true"`
	ParentVersion  string `col-name:"Direct\nDependency\nPackage\nVersion" auto-merge:"true"`
	PackageName    string `col-name:"Blocked\nPackage\nName" auto-merge:"true"`
	PackageVersion string `col-name:"Blocked\nPackage\nVersion" auto-merge:"true"`
	BlockingReason string `col-name:"Blocking Reason" auto-merge:"true"`
	PkgType        string `col-name:"Package\nType" auto-merge:"true"`
	Policy         string `col-name:"Violated\nPolicy\nName"`
	Condition      string `col-name:"Violated Condition\nName"`
	Explanation    string `col-name:"Explanation"`
	Recommendation string `col-name:"Recommendation"`
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
			if e := errorutils.CheckError(os.Chdir(rootDir)); err == nil {
				err = e
			}
		}()
	} else {
		ca.workingDirs = append(ca.workingDirs, rootDir)
	}
	results := map[string][]*PackageStatus{}
	for _, workDir := range ca.workingDirs {
		var absWd string
		absWd, err = filepath.Abs(workDir)
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
		if e := ca.doCurateAudit(results); e != nil {
			err = errors.Join(err, e)
		}
	}
	if ca.Progress() != nil {
		err = errors.Join(err, ca.Progress().Quit())
	}

	for projectPath, packagesStatus := range results {
		err = errors.Join(err, printResult(ca.OutputFormat(), projectPath, packagesStatus))
	}
	return
}

func (ca *CurationAuditCommand) doCurateAudit(results map[string][]*PackageStatus) error {
	techs := cmdUtils.DetectedTechnologies()
	for _, tech := range techs {
		if _, ok := supportedTech[coreutils.Technology(tech)]; !ok {
			log.Info(fmt.Sprintf(errorTemplateUnsupportedTech, tech))
			continue
		}
		if err := ca.auditTree(coreutils.Technology(tech), results); err != nil {
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
	if len(ca.FullDependenciesTree()) == 0 {
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
	_, projectName, projectScope, projectVersion := getUrlNameAndVersionByTech(tech, ca.FullDependenciesTree()[0].Id, "", "")
	if ca.Progress() != nil {
		ca.Progress().SetHeadlineMsg(fmt.Sprintf("Fetch curation status for %s graph with %v nodes project name: %s:%s", tech.ToFormal(), len(flattenGraph[0].Nodes)-1, projectName, projectVersion))
	}
	if projectScope != "" {
		projectName = projectScope + "/" + projectName
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
	rootNodeId := ca.FullDependenciesTree()[0].Id
	// Fetch status for each node from a flatten graph which, has no duplicate nodes.
	err = analyzer.fetchNodesStatus(flattenGraph[0], &packagesStatusMap, rootNodeId)
	analyzer.fillGraphRelations(ca.FullDependenciesTree()[0], &packagesStatusMap,
		&packagesStatus, "", "", datastructures.MakeSet[string](), true)
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
	log.Output(fmt.Sprintf("Found %v blocked packages for project %s", len(packagesStatus), projectPath))
	switch format {
	case utils.Json:
		if len(packagesStatus) > 0 {
			err := utils.PrintJson(packagesStatus)
			if err != nil {
				return err
			}
		}
	case utils.Table:
		pkgStatusTable := convertToPackageStatusTable(packagesStatus)
		err := coreutils.PrintTable(pkgStatusTable, "Curation", "Found 0 blocked packages", true)
		if err != nil {
			return err
		}
	}
	log.Output("\n")
	return nil
}

func convertToPackageStatusTable(packagesStatus []*PackageStatus) []PackageStatusTable {
	var pkgStatusTable []PackageStatusTable
	for index, pkgStatus := range packagesStatus {
		// We use auto-merge supported by the 'go-pretty' library. It doesn't have an option to merge lines by a group of unique fields.
		// In order to so, we make each group merge only with itself by adding or not adding space. This way, it won't be merged with the next group.
		uniqLineSep := ""
		if index%2 == 0 {
			uniqLineSep = " "
		}
		pkgTable := PackageStatusTable{
			ParentName:     pkgStatus.ParentName + uniqLineSep,
			ParentVersion:  pkgStatus.ParentVersion + uniqLineSep,
			PackageName:    pkgStatus.PackageName + uniqLineSep,
			PackageVersion: pkgStatus.PackageVersion + uniqLineSep,
			BlockingReason: pkgStatus.BlockingReason + uniqLineSep,
			PkgType:        pkgStatus.PkgType + uniqLineSep,
		}
		if len(pkgStatus.Policy) == 0 {
			pkgStatusTable = append(pkgStatusTable, pkgTable)
			continue
		}
		for _, policyCond := range pkgStatus.Policy {
			pkgTable.Policy = policyCond.Policy
			pkgTable.Explanation = policyCond.Explanation
			pkgTable.Recommendation = policyCond.Recommendation
			pkgTable.Condition = policyCond.Condition
			pkgStatusTable = append(pkgStatusTable, pkgTable)
		}
	}

	return pkgStatusTable
}

func (ca *CurationAuditCommand) CommandName() string {
	return "curation_audit"
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
				"project for the first time, the project should be configured using the 'jf npmc' command"))
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
		return errorutils.CheckErrorf(errorTemplateUnsupportedTech, tech.ToString())
	}
	return nil
}

func (nc *treeAnalyzer) fillGraphRelations(node *xrayUtils.GraphNode, preProcessMap *sync.Map,
	packagesStatus *[]*PackageStatus, parent, parentVersion string, visited *datastructures.Set[string], isRoot bool) {
	for _, child := range node.Nodes {
		packageUrl, name, scope, version := getUrlNameAndVersionByTech(nc.tech, child.Id, nc.url, nc.repo)
		if isRoot {
			parent = name
			parentVersion = version
			if scope != "" {
				parent = scope + "/" + parent
			}
		}
		if visited.Exists(scope + name + version + "-" + parent + parentVersion) {
			continue
		}

		visited.Add(scope + name + version + "-" + parent + parentVersion)
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
		nc.fillGraphRelations(child, preProcessMap, packagesStatus, parent, parentVersion, visited, false)
	}
}
func (nc *treeAnalyzer) fetchNodesStatus(graph *xrayUtils.GraphNode, p *sync.Map, rootNodeId string) error {
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
	packageUrl, name, scope, version := getUrlNameAndVersionByTech(nc.tech, node.Id, nc.url, nc.repo)
	if scope != "" {
		name = scope + "/" + name
	}
	resp, _, err := nc.rtManager.Client().SendHead(packageUrl, &nc.httpClientDetails)
	if err != nil {
		if resp != nil && resp.StatusCode >= 400 {
			return errorutils.CheckErrorf(errorTemplateHeadRequest, packageUrl, name, version, resp.StatusCode, err)
		}
		if resp == nil || resp.StatusCode != http.StatusForbidden {
			return err
		}
	}
	if resp != nil && resp.StatusCode >= 400 && resp.StatusCode != http.StatusForbidden {
		return errorutils.CheckErrorf(errorTemplateHeadRequest, packageUrl, name, version, resp.StatusCode, err)
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
			return nil, errorutils.CheckErrorf(errorTemplateHeadRequest, packageUrl, name, version, getResp.StatusCode, err)
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
// Message structure: Package %s:%s download was blocked by JFrog Packages Curation service due to the following policies violated {%s, %s, %s, %s},{%s, %s, %s, %s}.
func (nc *treeAnalyzer) extractPoliciesFromMsg(respError *ErrorsResp) []Policy {
	var policies []Policy
	msg := respError.Errors[0].Message
	allMatches := nc.extractPoliciesRegex.FindAllString(msg, -1)
	for _, match := range allMatches {
		match = strings.TrimSuffix(strings.TrimPrefix(match, "{"), "}")
		polCond := strings.Split(match, ",")
		if len(polCond) >= 2 {
			pol := polCond[0]
			cond := polCond[1]

			if len(polCond) == 4 {
				exp, rec := makeLegiblePolicyDetails(polCond[2], polCond[3])
				policies = append(policies, Policy{Policy: strings.TrimSpace(pol),
					Condition: strings.TrimSpace(cond), Explanation: strings.TrimSpace(exp), Recommendation: strings.TrimSpace(rec)})
				continue
			}
			policies = append(policies, Policy{Policy: strings.TrimSpace(pol), Condition: strings.TrimSpace(cond)})
		}
	}
	return policies
}

// Adding a new line after the headline and replace every "|" with a new line.
func makeLegiblePolicyDetails(explanation, recommendation string) (string, string) {
	explanation = strings.ReplaceAll(strings.Replace(explanation, ": ", ":\n", 1), " | ", "\n")
	recommendation = strings.ReplaceAll(strings.Replace(recommendation, ": ", ":\n", 1), " | ", "\n")
	return explanation, recommendation
}

func getUrlNameAndVersionByTech(tech coreutils.Technology, nodeId, artiUrl, repo string) (downloadUrl string, name string, scope string, version string) {
	if tech == coreutils.Npm {
		return getNpmNameScopeAndVersion(nodeId, artiUrl, repo, coreutils.Npm.ToString())
	}
	return
}

// The graph holds, for each node, the component ID (xray representation)
// from which we extract the package name, version, and construct the Artifactory download URL.
func getNpmNameScopeAndVersion(id, artiUrl, repo, tech string) (downloadUrl, name, scope, version string) {
	id = strings.TrimPrefix(id, tech+"://")

	nameVersion := strings.Split(id, ":")
	name = nameVersion[0]
	if len(nameVersion) > 1 {
		version = nameVersion[1]
	}
	scopeSplit := strings.Split(name, "/")
	if len(scopeSplit) > 1 {
		scope = scopeSplit[0]
		name = scopeSplit[1]
	}
	return buildNpmDownloadUrl(artiUrl, repo, name, scope, version), name, scope, version
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
