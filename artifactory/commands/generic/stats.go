package generic

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/access"
	"github.com/jfrog/jfrog-client-go/artifactory"
	clientUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientStats "github.com/jfrog/jfrog-client-go/artifactory/stats"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/pterm/pterm"
	"strings"
)

const displayLimit = 5

type Stats struct {
	ServicesManager artifactory.ArtifactoryServicesManager
	AccessManager   access.AccessServicesManager
	ProductName     string
	FormatOutput    string
	AccessToken     string
	ServerId        string
	ServerUrl       string
}

func NewStatsCommand() *Stats {
	return &Stats{}
}
func (s *Stats) SetServicesManager(manager artifactory.ArtifactoryServicesManager) *Stats {
	s.ServicesManager = manager
	return s
}

func (s *Stats) SetProductName(name string) *Stats {
	s.ProductName = name
	return s
}

func (s *Stats) SetFormatOutput(format string) *Stats {
	s.FormatOutput = format
	return s
}

func (s *Stats) SetAccessToken(token string) *Stats {
	s.AccessToken = token
	return s
}

func (s *Stats) SetServerId(id string) *Stats {
	s.ServerId = id
	return s
}

func (ss *Stats) Run() error {
	serverDetails, err := config.GetSpecificConfig(ss.ServerId, true, false)
	if err != nil {
		return err
	}

	servicesManager, err := utils.CreateServiceManager(serverDetails, -1, 0, false)
	if err != nil {
		return err
	}
	ss.ServicesManager = servicesManager
	serverDetails.AccessToken = ss.AccessToken

	accessManager, err := utils.CreateAccessServiceManager(serverDetails, false)
	if err != nil {
		return err
	}
	ss.AccessManager = *accessManager

	err = ss.GetStats(serverDetails.GetUrl())
	if err != nil {
		return err
	}
	return nil
}

type ArtifactoryInfo struct {
	StorageInfo         clientUtils.StorageInfo
	RepositoriesDetails []RepositoryDetails `json:"-"`
	ProjectsCount       int
}

type ProjectResources struct {
	Resources []Resource `json:"resources"`
}

type Resource struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	BinMgrID string `json:"bin_mgr_id"`
}

type Project struct {
	DisplayName     string          `json:"display_name"`
	Description     string          `json:"description"`
	AdminPrivileges AdminPrivileges `json:"admin_privileges"`
	ProjectKey      string          `json:"project_key"`
}

type AdminPrivileges struct {
	ManageMembers   bool `json:"manage_members"`
	ManageResources bool `json:"manage_resources"`
	IndexResources  bool `json:"index_resources"`
}

type JPD struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	URL      string    `json:"base_url"`
	Status   Status    `json:"status"`
	Local    bool      `json:"local"`
	Services []Service `json:"services"`
	Licenses []License `json:"licenses"`
}

type Status struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Service struct {
	Type   string `json:"type"`
	Status Status `json:"status"`
}

type License struct {
	Type       string `json:"type"`
	Expired    bool   `json:"expired"`
	LicensedTo string `json:"licensed_to"`
}

type ReleaseBundleResponse struct {
	ReleaseBundles []ReleaseBundleInfo `json:"release_bundles"`
}

type ReleaseBundleInfo struct {
	RepositoryKey     string `json:"repository_key"`
	ReleaseBundleName string `json:"release_bundle_name"`
	ProjectKey        string `json:"project_key"`
}

type RepositoryDetails struct {
	Key         string `json:"key"`
	Type        string `json:"type"`
	PackageType string `json:"packageType"`
}

type GenericResultsWriter struct {
	data   interface{}
	format string
}

func NewGenericResultsWriter(data interface{}, format string) *GenericResultsWriter {
	return &GenericResultsWriter{
		data:   data,
		format: format,
	}
}

type StatsFunc func(string) (interface{}, error)

func (ss Stats) GetCommandMap() map[string]StatsFunc {
	return map[string]StatsFunc{
		"rb":  ss.GetReleaseBundlesStats,
		"jpd": ss.GetJPDsStats,
		"rt":  ss.GetArtifactoryStats,
		"pj":  ss.GetProjectsStats,
	}
}

var needAdminTokenMap = map[string]bool{
	"PROJECTS": true,
	"JPD":      true,
}

var processingOrders = []string{"pj", "rt", "jpd", "rb"}

var printingOrders = []string{"pj", "rt", "jpd", "rb"}

func (ss Stats) GetStats(serverUrl string) error {

	commandMap := ss.GetCommandMap()

	allResultsMap := make(map[string]interface{})

	product := ss.ProductName

	if product != "" {
		if commandFunc, ok := commandMap[product]; ok {
			allResultsMap[product] = GetStatsForProduct(commandFunc, serverUrl)
			if product == "rt" {
				allResultsMap["pj"] = GetStatsForProduct(commandMap["pj"], serverUrl)
				updateProjectInArtifactory(&allResultsMap)
				delete(allResultsMap, "pj")
			}
		} else {
			return fmt.Errorf("unknown product: %s", product)
		}
	} else {
		for _, product := range processingOrders {
			allResultsMap[product] = GetStatsForProduct(commandMap[product], serverUrl)
		}
		updateProjectInArtifactory(&allResultsMap)
	}
	return ss.PrintAllResults(allResultsMap)
}

func (ss Stats) PrintAllResults(results map[string]interface{}) error {
	for _, product := range printingOrders {
		result, ok := results[product]
		if ok {
			err := NewGenericResultsWriter(result, ss.FormatOutput).Print()
			if err != nil {
				log.Error("Failed to print result:", err)
			}
		}
	}
	return nil
}

func GetStatsForProduct(commandAPI StatsFunc, serverUrl string) interface{} {
	body, err := commandAPI(serverUrl)
	if err != nil {
		return err
	}
	return body
}

func (rw *GenericResultsWriter) Print() error {
	switch rw.format {
	case "json", "simplejson":
		return rw.printJson()
	case "table":
		return rw.printDashboard()
	default:
		return rw.printSimple()
	}
}

func (rw *GenericResultsWriter) printJson() error {
	if rw.data == nil {
		return nil
	}

	jsonBytes, err := json.MarshalIndent(rw.data, "", "  ")
	if err != nil {
		return err
	}
	result := string(jsonBytes)
	if len(result) <= 2 {
		msg := ""
		switch rw.data.(type) {
		case *ArtifactoryInfo:
			msg = "Artifacts: No Artifacts Available"
		case []Project:
			msg = "Projects: No Project Available"
		case *[]JPD:
			msg = "JPDs: No JPD Available"
		case *ReleaseBundleResponse:
			msg = "Release Bundles: No Release Bundle Info Available"
		case clientStats.APIError:
			msg = fmt.Sprintf("API Errors: %s", rw.data.(error).Error())
		case clientStats.GenericError:
			msg = fmt.Sprintf("Errors: %s", rw.data.(error).Error())
		}
		jsonBytes, err = json.MarshalIndent(msg, "", "  ")
		if err != nil {
			return err
		}
		result = string(jsonBytes)
	}
	fmt.Println(result)
	return nil
}

func (rw *GenericResultsWriter) printDashboard() error {
	if rw.data == nil {
		return nil
	}

	switch v := rw.data.(type) {
	case *ArtifactoryInfo:
		printArtifactoryDashboard(v)
	case []Project:
		printProjectsDashboard(v)
	case *[]JPD:
		printJPDsDashboard(v)
	case *ReleaseBundleResponse:
		printReleaseBundlesDashboard(v)
	case *clientStats.APIError:
		printErrorDashboard(v)
	case *clientStats.GenericError:
		printGenericErrorDashboard(v)
	}
	return nil
}

func printArtifactoryDashboard(stats *ArtifactoryInfo) {
	pterm.Println("📦 Artifactory Summary")

	summaryTableData := pterm.TableData{
		{"Metric", "Value"},
		{"Total Projects", pterm.LightCyan(stats.ProjectsCount)},
		{"Total Binaries", pterm.LightCyan(stats.StorageInfo.BinariesCount)},
		{"Total Binaries Size", pterm.LightCyan(stats.StorageInfo.BinariesSize)},
		{"Total Artifacts ", pterm.LightCyan(stats.StorageInfo.ArtifactsCount)},
		{"Total Artifacts Size", pterm.LightCyan(stats.StorageInfo.ArtifactsSize)},
		{"Storage Type", pterm.LightCyan(stats.StorageInfo.StorageType)},
	}
	pterm.DefaultTable.WithHasHeader().WithData(summaryTableData).Render()

	repoTypeCounts := make(map[string]int)

	for _, repo := range stats.RepositoriesDetails {
		if repo.Type != "TOTAL" && repo.Type != "NA" {
			repoTypeCounts[repo.Type]++
		}
	}

	breakdownData := pterm.TableData{{"Repository Type", "Count"}}
	for repoType, count := range repoTypeCounts {
		breakdownData = append(breakdownData, []string{pterm.LightMagenta(repoType), pterm.LightGreen(fmt.Sprintf("%d", count))})
	}
	pterm.DefaultTable.WithHasHeader().WithData(breakdownData).Render()
}

func printProjectsDashboard(projects []Project) {
	pterm.Println("📁 Projects")
	if len(projects) == 0 {
		pterm.Warning.Println("No Projects found.")
		return
	}
	loopRange := len(projects)
	if loopRange > displayLimit {
		loopRange = displayLimit
	}
	actualProjectsCount := len(projects)

	tableData := pterm.TableData{{"Project Key", "Display Name"}}
	for i := 0; i < loopRange; i++ {
		project := (projects)[i]
		tableData = append(tableData, []string{pterm.LightBlue(project.ProjectKey), project.DisplayName})
	}

	tableString, _ := pterm.DefaultTable.WithHasHeader().WithData(tableData).Srender()
	trimmedTable := strings.TrimSuffix(tableString, "\n")

	pterm.Print(trimmedTable)
	if actualProjectsCount > displayLimit {
		pterm.Println(pterm.Yellow(fmt.Sprintf("\n...and %d more projects. Refer JSON output format for complete list.", actualProjectsCount-displayLimit)))
	}
	pterm.Print("\n")
}

func printJPDsDashboard(jpdList *[]JPD) {
	pterm.Println("🛰️ JFrog Platform Deployments (JPDs)")
	if jpdList == nil || len(*jpdList) == 0 {
		pterm.Warning.Println("No JPDs found.")
		pterm.Println()
		return
	}

	loopRange := len(*jpdList)
	if loopRange > displayLimit {
		loopRange = displayLimit
	}
	actualCount := len(*jpdList)

	tableData := pterm.TableData{{"Name", "URL", "Status"}}
	for i := 0; i < loopRange; i++ {
		jpd := (*jpdList)[i]
		var status string
		if jpd.Status.Code == "ONLINE" {
			status = pterm.LightGreen(jpd.Status.Code)
		} else {
			status = pterm.LightRed(jpd.Status.Code)
		}
		tableData = append(tableData, []string{pterm.LightCyan(jpd.Name), jpd.URL, status})
	}

	tableString, _ := pterm.DefaultTable.WithHasHeader().WithData(tableData).Srender()
	pterm.Print(strings.TrimSuffix(tableString, "\n"))

	if actualCount > displayLimit {
		pterm.Print(pterm.Yellow(fmt.Sprintf("\n...and %d more JPDs. Refer JSON output format for complete list.\n", actualCount-displayLimit)))
	}
	pterm.Print("\n\n")
}

func printReleaseBundlesDashboard(rbResponse *ReleaseBundleResponse) {
	pterm.Println("🚀 Release Bundles")
	if rbResponse == nil || len(rbResponse.ReleaseBundles) == 0 {
		pterm.Warning.Println("No Release Bundles found.")
		pterm.Println()
		return
	}

	loopRange := len(rbResponse.ReleaseBundles)
	if loopRange > displayLimit {
		loopRange = displayLimit
	}
	actualCount := len(rbResponse.ReleaseBundles)

	tableData := pterm.TableData{{"Release Bundle Name", "Project Key", "Repository Key"}}
	for i := 0; i < loopRange; i++ {
		rb := rbResponse.ReleaseBundles[i]
		tableData = append(tableData, []string{
			pterm.LightGreen(rb.ReleaseBundleName),
			rb.ProjectKey,
			pterm.LightYellow(rb.RepositoryKey),
		})
	}

	tableString, _ := pterm.DefaultTable.WithHasHeader().WithData(tableData).Srender()
	pterm.Print(strings.TrimSuffix(tableString, "\n"))

	if actualCount > displayLimit {
		pterm.Print(pterm.Yellow(fmt.Sprintf("\n...and %d more release bundles. Refer JSON output format for complete list.\n", actualCount-displayLimit)))
	}
	pterm.Print("\n\n")
}

func printErrorDashboard(apiError *clientStats.APIError) {
	_, ok := needAdminTokenMap[apiError.Product]
	Suggestion := ""
	if apiError.StatusCode >= 400 && apiError.StatusCode < 500 && ok {
		Suggestion = "Need Admin Token"
	} else {
		Suggestion = apiError.Suggestion
	}

	tableData := pterm.TableData{
		{"Product: ", apiError.Product},
		{"Status Code", pterm.LightRed(fmt.Sprintf("%d", apiError.StatusCode))},
		{"Status", pterm.LightRed(apiError.StatusText)},
		{"Suggestion", pterm.LightYellow(Suggestion)},
	}
	pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()
}

func printGenericErrorDashboard(err *clientStats.GenericError) {
	tableData := pterm.TableData{
		{err.Product, err.Err},
	}
	pterm.DefaultTable.WithBoxed(true).WithData(tableData).Render()
}

func (rw *GenericResultsWriter) printSimple() error {
	if rw.data == nil {
		return nil
	}

	switch v := rw.data.(type) {
	case *ArtifactoryInfo:
		printArtifactoryStats(v)
	case []Project:
		printProjectsStats(v)
	case *[]JPD:
		printJPDsStats(v)
	case *ReleaseBundleResponse:
		printReleaseBundlesSimple(v)
	case *clientStats.APIError:
		printErrorMessage(v)
	case *clientStats.GenericError:
		printGenericError(v)
	}
	return nil
}

func printReleaseBundlesSimple(rbResponse *ReleaseBundleResponse) {
	log.Output("--- Release Bundles ---")
	if len(rbResponse.ReleaseBundles) == 0 {
		log.Output("No Release Bundles Available")
		log.Output()
		return
	}
	loopRange := len(rbResponse.ReleaseBundles)
	if loopRange > displayLimit {
		loopRange = displayLimit
	}
	actualProjectsCount := len(rbResponse.ReleaseBundles)
	for i := 0; i < loopRange; i++ {
		rb := rbResponse.ReleaseBundles[i]
		log.Output("ReleaseBundle: ", i+1)
		log.Output("ReleaseBundleName: ", rb.ReleaseBundleName)
		log.Output("RepositoryKey: ", rb.RepositoryKey)
		log.Output("ProjectKey:", rb.ProjectKey)
		log.Output()
	}
	if actualProjectsCount > displayLimit {
		log.Output(pterm.Yellow(fmt.Sprintf("...and %d more release bundles, try JSON format for complete list", actualProjectsCount-displayLimit)))
	}
	log.Output()
}

func printArtifactoryStats(stats *ArtifactoryInfo) {
	log.Output("--- Artifactory Statistics ---")
	log.Output("Total Projects: ", stats.ProjectsCount)
	log.Output("Total No of Binaries: ", stats.StorageInfo.BinariesCount)
	log.Output("Total Binaries Size: ", stats.StorageInfo.BinariesSize)
	log.Output("Total No of Artifacts: ", stats.StorageInfo.ArtifactsCount)
	log.Output("Total Artifacts Size: ", stats.StorageInfo.ArtifactsSize)
	log.Output("Storage Type: ", stats.StorageInfo.StorageType)
	log.Output()

	repoTypeCounts := make(map[string]int)
	for _, repo := range stats.RepositoriesDetails {
		if repo.Type != "TOTAL" && repo.Type != "NA" {
			repoTypeCounts[repo.Type]++
		}
	}
	log.Output("--- Repositories Details ---")
	for repoType, count := range repoTypeCounts {
		log.Output(repoType, ": ", count)
	}
	log.Output()
}

func printProjectsStats(projects []Project) {
	log.Output("--- Available Projects ---")
	if len(projects) == 0 {
		log.Output("No Projects Available")
		log.Output()
		return
	}
	loopRange := len(projects)
	if loopRange > displayLimit {
		loopRange = displayLimit
	}
	actualProjectsCount := len(projects)
	for i := 0; i < loopRange; i++ {
		project := (projects)[i]
		log.Output("Project: ", i+1)
		log.Output("Name: ", project.DisplayName)
		log.Output("Key: ", project.ProjectKey)
		if project.Description != "" {
			log.Output("Description: ", project.Description)
		} else {
			log.Output("Description: NA")
		}
		log.Output()
	}
	if actualProjectsCount > displayLimit {
		log.Output(pterm.Yellow(fmt.Sprintf("...and %d more projects, try JSON format for complete list", actualProjectsCount-displayLimit)))
	}
	log.Output()
}

func printJPDsStats(jpdList *[]JPD) {
	log.Output("--- Available JPDs ---")
	if len(*jpdList) == 0 {
		log.Output("No JPDs Info Available")
		log.Output()
		return
	}
	loopRange := len(*jpdList)
	if loopRange > displayLimit {
		loopRange = displayLimit
	}
	actualProjectsCount := len(*jpdList)
	for i := 0; i < loopRange; i++ {
		jpd := (*jpdList)[i]
		log.Output("JPD: ", i+1)
		log.Output("Name: ", jpd.Name)
		log.Output("URL: ", jpd.URL)
		log.Output("Status: ", jpd.Status.Code)
		log.Output("Detailed Status: ", jpd.Status.Message)
		log.Output("Local: ", jpd.Local)
		log.Output()
	}
	if actualProjectsCount > displayLimit {
		log.Output(pterm.Yellow(fmt.Sprintf("...and %d more JPDs, try JSON format for complete list", actualProjectsCount-displayLimit)))
	}
}

func printErrorMessage(apiError *clientStats.APIError) {
	_, ok := needAdminTokenMap[apiError.Product]
	Suggestion := ""
	if apiError.StatusCode == 401 && ok {
		Suggestion = "Need Admin Token"
	} else {
		Suggestion = apiError.Suggestion
	}
	log.Output("---", apiError.Product, "---")
	log.Output("StatusCode - ", apiError.StatusCode)
	log.Output("StatusText - ", apiError.StatusText)
	log.Output("Suggestion - ", Suggestion)
	log.Output()
}

func printGenericError(err *clientStats.GenericError) {
	_, ok := needAdminTokenMap[err.Product]
	Suggestion := ""
	if ok {
		Suggestion = "Need Admin Token"
	} else {
		Suggestion = err.Err
	}
	log.Output("---", err.Product, "---")
	log.Output("Error: ", Suggestion)
	log.Output()
}

func (ss Stats) GetArtifactoryStats(serverUrl string) (interface{}, error) {
	var artifactoryStats ArtifactoryInfo
	storageInfo, err := ss.ServicesManager.GetStorageInfo()
	if err != nil {
		return nil, err
	}
	artifactoryStats.StorageInfo = *storageInfo
	body, err := ss.ServicesManager.GetRepositoriesStats(serverUrl)
	if err != nil {
		return nil, err
	}
	var repositoriesDetails []RepositoryDetails
	if err := json.Unmarshal(body, &repositoriesDetails); err != nil {
		return nil, fmt.Errorf("error parsing repositories JSON: %w", err)
	}
	artifactoryStats.RepositoriesDetails = repositoriesDetails
	return &artifactoryStats, nil
}

func (ss Stats) GetProjectsStats(serverUrl string) (interface{}, error) {
	projects, err := ss.AccessManager.GetProjectsStats()
	if err != nil {
		return nil, err
	}
	return projects, nil
}

func (ss Stats) GetJPDsStats(serverUrl string) (interface{}, error) {
	body, err := ss.ServicesManager.GetJPDsStats(serverUrl)
	if err != nil {
		return nil, err
	}
	var jpdList []JPD
	if err := json.Unmarshal(body, &jpdList); err != nil {
		return nil, fmt.Errorf("error parsing JPDs JSON: %w", err)
	}
	return &jpdList, nil
}

func (ss Stats) GetReleaseBundlesStats(serverUrl string) (interface{}, error) {
	body, err := ss.ServicesManager.GetReleaseBundlesStats(serverUrl)
	if err != nil {
		return nil, err
	}
	var releaseBundles ReleaseBundleResponse
	if err := json.Unmarshal(body, &releaseBundles); err != nil {
		return nil, fmt.Errorf("error parsing ReleaseBundles JSON: %w", err)
	}
	return &releaseBundles, nil
}

func updateProjectInArtifactory(allResultsMap *map[string]interface{}) {
	m := *allResultsMap

	pjResult, pjOk := m["pj"]
	if !pjOk || pjResult == nil {
		return
	}

	rtResult, rtOk := m["rt"]
	if !rtOk || rtResult == nil {
		return
	}

	projects, ok := pjResult.([]Project)
	if !ok {
		return
	}

	artifactoryInfo, ok := rtResult.(*ArtifactoryInfo)
	if !ok {
		return
	}

	artifactoryInfo.ProjectsCount = len(projects)
	m["rt"] = artifactoryInfo
}
