package coreStats

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	clientStats "github.com/jfrog/jfrog-client-go/utils/stats"
	"github.com/pterm/pterm"
	"strings"
)

const displayLimit = 5

type ArtifactoryInfo struct {
	BinariesSummary         BinariesSummary     `json:"binariesSummary"`
	FileStoreSummary        FileStoreSummary    `json:"fileStoreSummary"`
	RepositoriesSummaryList []RepositorySummary `json:"repositoriesSummaryList"`
	ProjectsCount           int                 `json:"-"`
	RepositoriesDetails     []RepositoryDetails `json:"-"`
}

type BinariesSummary struct {
	BinariesCount  string `json:"binariesCount"`
	BinariesSize   string `json:"binariesSize"`
	ArtifactsCount string `json:"artifactsCount"`
	ArtifactsSize  string `json:"artifactsSize"`
}

type FileStoreSummary struct {
	StorageType      string `json:"storageType"`
	StorageDirectory string `json:"storageDirectory"`
}

type RepositorySummary struct {
	RepoKey     string `json:"repoKey"`
	RepoType    string `json:"repoType"`
	PackageType string `json:"packageType"`
}

type XrayPolicy struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Author   string `json:"author"`
	Rules    []Rule `json:"rules"`
	Created  string `json:"created"`
	Modified string `json:"modified"`
}

type Rule struct {
	Name     string   `json:"name"`
	Priority int      `json:"priority"`
	Actions  struct{} `json:"actions"`
	Criteria Criteria `json:"criteria"`
}

type Criteria struct {
	MinSeverity string `json:"min_severity"`
}

type XrayWatch struct {
	GeneralData      GeneralData      `json:"general_data"`
	ProjectResources ProjectResources `json:"project_resources"`
}

type GeneralData struct {
	ID   string `json:"id"`
	Name string `json:"name"`
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

type StatsFunc func(client *httpclient.HttpClient, artifactoryUrl string, hd httputils.HttpClientDetails) (interface{}, error)

func getCommandMap() map[string]StatsFunc {
	return map[string]StatsFunc{
		"rb":  GetReleaseBundlesStats,
		"jpd": GetJPDsStats,
		"rt":  GetArtifactoryStats,
		"pj":  GetProjectsStats,
	}
}

var needAdminTokenMap = map[string]bool{
	"PROJECTS": true,
	"JPD":      true,
}

func GetStats(outputFormat string, product string, accessToken string, serverId string) error {
	serverDetails, err := config.GetSpecificConfig(serverId, true, false)
	if err != nil {
		return err
	}

	httpClientDetails := httputils.HttpClientDetails{AccessToken: serverDetails.AccessToken, User: serverDetails.User, Password: serverDetails.Password}
	if accessToken != "" {
		httpClientDetails.AccessToken = accessToken
	}

	httpClientDetails.SetContentTypeApplicationJson()
	client, err := httpclient.ClientBuilder().Build()
	if err != nil {
		return err
	}

	commandMap := getCommandMap()

	serverUrl := serverDetails.GetUrl()

	allResultsMap := make(map[string]interface{})

	if product != "" {
		if commandFunc, ok := commandMap[product]; ok {
			allResultsMap[product] = GetStatsForProduct(commandFunc, client, serverUrl, httpClientDetails)
			if product == "rt" {
				allResultsMap["pj"] = GetStatsForProduct(commandMap["pj"], client, serverUrl, httpClientDetails)
				updateProjectInArtifactory(allResultsMap)
				delete(allResultsMap, "pj")
			}
		} else {
			return fmt.Errorf("unknown product: %s", product)
		}
	} else {
		for product, productAPI := range commandMap {
			allResultsMap[product] = GetStatsForProduct(productAPI, client, serverUrl, httpClientDetails)
		}
		updateProjectInArtifactory(allResultsMap)
	}

	return printAllResults(allResultsMap, outputFormat)
}

func printAllResults(results map[string]interface{}, outputFormat string) error {
	productOrder := []string{"rt", "jpd", "pj", "rb"}
	for _, product := range productOrder {
		result, ok := results[product]
		if ok {
			err := NewGenericResultsWriter(result, outputFormat).Print()
			if err != nil {
				log.Error("Failed to print result:", err)
			}
		}
	}
	return nil
}

func GetStatsForProduct(commandAPI StatsFunc, client *httpclient.HttpClient, artifactoryUrl string, httpClientDetails httputils.HttpClientDetails) interface{} {
	body, err := commandAPI(client, artifactoryUrl, httpClientDetails)
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
		case *[]Project:
			msg = "Projects: No Project Available"
		case *[]JPD:
			msg = "JPDs: No JPD Available"
		case *ReleaseBundleResponse:
			msg = "Release Bundles: No Release Bundle Info Available"
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
	case *[]Project:
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
		{"Total Binaries", pterm.LightCyan(stats.BinariesSummary.BinariesCount)},
		{"Total Binaries Size", pterm.LightCyan(stats.BinariesSummary.BinariesSize)},
		{"Total Artifacts ", pterm.LightCyan(stats.BinariesSummary.ArtifactsCount)},
		{"Total Artifacts Size", pterm.LightCyan(stats.BinariesSummary.ArtifactsSize)},
		{"Storage Type", pterm.LightCyan(stats.FileStoreSummary.StorageType)},
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

func printProjectsDashboard(projects *[]Project) {
	pterm.Println("📁 Projects")
	if len(*projects) == 0 {
		pterm.Warning.Println("No Projects found.")
		return
	}
	loopRange := len(*projects)
	if loopRange > displayLimit {
		loopRange = displayLimit
	}
	actualProjectsCount := len(*projects)

	tableData := pterm.TableData{{"Project Key", "Display Name"}}
	for i := 0; i < loopRange; i++ {
		project := (*projects)[i]
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
		{err.Product, err.Err.Error()},
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
	case *[]Project:
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
	log.Output("Total No of Binaries: ", stats.BinariesSummary.BinariesCount)
	log.Output("Total Binaries Size: ", stats.BinariesSummary.BinariesSize)
	log.Output("Total No of Artifacts: ", stats.BinariesSummary.ArtifactsCount)
	log.Output("Total Artifacts Size: ", stats.BinariesSummary.ArtifactsSize)
	log.Output("Storage Type: ", stats.FileStoreSummary.StorageType)
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

func printProjectsStats(projects *[]Project) {
	log.Output("--- Available Projects ---")
	if len(*projects) == 0 {
		log.Output("No Projects Available")
		log.Output()
		return
	}
	loopRange := len(*projects)
	if loopRange > displayLimit {
		loopRange = displayLimit
	}
	actualProjectsCount := len(*projects)
	for i := 0; i < loopRange; i++ {
		project := (*projects)[i]
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
	if apiError.StatusCode >= 400 && apiError.StatusCode < 500 && ok {
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
	log.Output("---", err.Product, "---")
	log.Output("Error:  ", err.Err.Error())
	log.Output()
}

func GetArtifactoryStats(client *httpclient.HttpClient, serverUrl string, httpClientDetails httputils.HttpClientDetails) (interface{}, error) {
	body, err := clientStats.GetArtifactoryStats(client, serverUrl, httpClientDetails)
	if err != nil {
		return nil, err
	}
	var stats ArtifactoryInfo
	if err := json.Unmarshal(body, &stats); err != nil {
		return nil, fmt.Errorf("error parsing Artifactory JSON: %w", err)
	}

	body, err = clientStats.GetRepositoriesStats(client, serverUrl, httpClientDetails)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(body, &stats.RepositoriesDetails); err != nil {
		return nil, fmt.Errorf("error parsing repositories JSON: %w", err)
	}
	return &stats, nil
}

func GetProjectsStats(client *httpclient.HttpClient, serverUrl string, httpClientDetails httputils.HttpClientDetails) (interface{}, error) {
	body, err := clientStats.GetProjectsStats(client, serverUrl, httpClientDetails)
	if err != nil {
		return nil, err
	}
	var projects []Project
	if err := json.Unmarshal(body, &projects); err != nil {
		return nil, fmt.Errorf("error parsing Projects JSON: %w", err)
	}
	return &projects, nil
}

func GetJPDsStats(client *httpclient.HttpClient, serverUrl string, httpClientDetails httputils.HttpClientDetails) (interface{}, error) {
	body, err := clientStats.GetJPDsStats(client, serverUrl, httpClientDetails)
	if err != nil {
		return nil, err
	}
	var jpdList []JPD
	if err := json.Unmarshal(body, &jpdList); err != nil {
		return nil, fmt.Errorf("error parsing JPDs JSON: %w", err)
	}
	return &jpdList, nil
}

func GetReleaseBundlesStats(client *httpclient.HttpClient, serverUrl string, httpClientDetails httputils.HttpClientDetails) (interface{}, error) {
	body, err := clientStats.GetReleaseBundlesStats(client, serverUrl, httpClientDetails)
	if err != nil {
		return nil, err
	}
	var releaseBundles ReleaseBundleResponse
	if err := json.Unmarshal(body, &releaseBundles); err != nil {
		return nil, fmt.Errorf("error parsing ReleaseBundles JSON: %w", err)
	}
	return &releaseBundles, nil
}

func updateProjectInArtifactory(allResultsMap map[string]interface{}) {
	projectsCount := 0
	if allResultsMap["pj"] != nil && allResultsMap["rt"] != nil {
		projects := allResultsMap["pj"].(*[]Project)
		projectsCount = len(*projects)
		artifactoryInfo := allResultsMap["rt"].(*ArtifactoryInfo)
		artifactoryInfo.ProjectsCount = projectsCount
		allResultsMap["rt"] = artifactoryInfo
	}
}
