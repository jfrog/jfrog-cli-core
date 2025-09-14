package coreStats

import (
	"encoding/json"
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	clientStats "github.com/jfrog/jfrog-client-go/utils/stats"
	"os"
)

type ArtifactoryInfo struct {
	BinariesSummary         BinariesSummary     `json:"binariesSummary"`
	FileStoreSummary        FileStoreSummary    `json:"fileStoreSummary"`
	RepositoriesSummaryList []RepositorySummary `json:"repositoriesSummaryList"`
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

type StatsFunc func(client *httpclient.HttpClient, sd *config.ServerDetails, hd httputils.HttpClientDetails) (interface{}, error)

func getCommandMap() map[string]StatsFunc {
	return map[string]StatsFunc{
		"rt":  GetArtifactoryStats,
		"rpr": GetRepositoriesStats,
		"xrp": GetXrayPolicies,
		"xrw": GetXrayWatches,
		"pr":  GetProjectsStats,
		"rb":  GetReleaseBundlesStats,
		"jpd": GetJPDsStats,
	}
}

func GetStats(outputFormat string, product string, accessToken string) error {
	serverDetails, err := config.GetDefaultServerConf()
	if err != nil {
		return err
	}

	httpClientDetails := httputils.HttpClientDetails{AccessToken: serverDetails.AccessToken}
	if accessToken != "" {
		httpClientDetails.AccessToken = accessToken
	}

	httpClientDetails.SetContentTypeApplicationJson()
	client, err := httpclient.ClientBuilder().Build()
	if err != nil {
		return err
	}

	commandMap := getCommandMap()

	commandFunc, ok := commandMap[product]

	if product != "" {
		if !ok {
			err = fmt.Errorf("unknown product: %s", product)
			return err
		} else {
			_ = GetStatsForProduct(commandFunc, product, outputFormat, client, serverDetails, httpClientDetails)
		}
	} else {
		for productName, commandAPI := range commandMap {
			_ = GetStatsForProduct(commandAPI, productName, outputFormat, client, serverDetails, httpClientDetails)
		}
	}

	return nil
}

func GetStatsForProduct(commandAPI StatsFunc, productName string, outputFormat string, client *httpclient.HttpClient, serverDetails *config.ServerDetails, httpClientDetails httputils.HttpClientDetails) error {
	body, err := commandAPI(client, serverDetails, httpClientDetails)
	if err != nil {
		err = NewGenericResultsWriter(err, outputFormat).Print()
		if err != nil {
			log.Error(productName, " : ", err)
			return err
		}
	} else {
		err := NewGenericResultsWriter(body, outputFormat).Print()
		if err != nil {
			log.Error(productName, " : ", err)
			return err
		}
	}
	return nil
}

func (rw *GenericResultsWriter) Print() error {
	switch rw.format {
	case "json", "simplejson":
		return rw.printJson()
	case "table":
		return rw.printTable()
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
		case *[]RepositoryDetails:
			msg = "Repositories: No Repository Available"
		case *[]XrayPolicy:
			msg = "Policies: No Xray Policy Available"
		case *[]XrayWatch:
			msg = "Watches: No Xray Watch Available"
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

func (rw *GenericResultsWriter) printTable() error {
	if rw.data == nil {
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleDouble)

	switch v := rw.data.(type) {
	case *ArtifactoryInfo:
		printArtifactoryStatsTable(t, v)
	case *[]XrayPolicy:
		printXrayPoliciesTable(t, v)
	case *[]XrayWatch:
		printXrayWatchesTable(t, v)
	case *[]Project:
		printProjectsTable(t, v)
	case *[]JPD:
		printJPDsTable(t, v)
	case *ReleaseBundleResponse:
		printReleaseBundlesTable(t, v)
	case *[]RepositoryDetails:
		printRepositoriesTable(t, v)
	default:
		if apiErr, ok := v.(*clientStats.APIError); ok {
			printErrorTable(t, apiErr)
		} else {
			log.Warn("Table format is not supported for this unknown data type.")
		}
	}
	t.Render()
	log.Output()
	return nil
}

func printArtifactoryStatsTable(t table.Writer, stats *ArtifactoryInfo) {
	t.AppendHeader(table.Row{"ARTIFACTS METRIC", "COUNT"})
	t.AppendRows([]table.Row{
		{"Total No of Artifacts", stats.BinariesSummary.ArtifactsCount},
		{"Total Binaries Size:", stats.BinariesSummary.BinariesSize},
		{"Total Storage Used: ", stats.BinariesSummary.ArtifactsSize},
		{"Storage Type: ", stats.FileStoreSummary.StorageType},
	})
	t.Render()
	t.ResetRows()
	t.ResetHeaders()
}

func printXrayPoliciesTable(t table.Writer, policies *[]XrayPolicy) {
	if len(*policies) == 0 {
		t.AppendRow(table.Row{"No Policy Available"})
		return
	}
	t.AppendHeader(table.Row{"Policy Name", "Type", "Author"})
	for _, policy := range *policies {
		t.AppendRow(table.Row{policy.Name, policy.Type, policy.Author})
	}
}

func printXrayWatchesTable(t table.Writer, watches *[]XrayWatch) {
	if len(*watches) == 0 {
		t.AppendRow(table.Row{"No Watches Available"})
		return
	}
	t.AppendHeader(table.Row{"Watch Name"})
	for _, watch := range *watches {
		t.AppendRow(table.Row{watch.GeneralData.Name})
	}
}

func printProjectsTable(t table.Writer, projects *[]Project) {
	if len(*projects) == 0 {
		t.AppendRow(table.Row{"No Projects Available"})
		return
	}
	t.AppendHeader(table.Row{"Project Key", "Display Name"})
	for _, project := range *projects {
		t.AppendRow(table.Row{project.ProjectKey, project.DisplayName})
	}
}

func printJPDsTable(t table.Writer, jpdList *[]JPD) {
	if len(*jpdList) == 0 {
		t.AppendRow(table.Row{"No JPDs Available"})
		return
	}
	t.AppendHeader(table.Row{"JPD Name", "URL", "Status"})
	for _, jpd := range *jpdList {
		t.AppendRow(table.Row{jpd.Name, jpd.URL, jpd.Status.Code})
	}
}

func printErrorTable(t table.Writer, apiError *clientStats.APIError) {
	t.AppendHeader(table.Row{"Product", "Status", "Text", "Suggestion"})
	t.AppendRow(table.Row{apiError.Product, apiError.StatusCode, apiError.StatusText, apiError.Suggestion})
}

func (rw *GenericResultsWriter) printSimple() error {
	if rw.data == nil {
		return nil
	}

	switch v := rw.data.(type) {
	case *ArtifactoryInfo:
		printArtifactoryStats(v)
	case *[]XrayPolicy:
		printXrayPoliciesStats(v)
	case *[]XrayWatch:
		printXrayWatchesStats(v)
	case *[]Project:
		printProjectsStats(v)
	case *[]JPD:
		printJPDsStats(v)
	case *ReleaseBundleResponse:
		printReleaseBundlesSimple(v)
	case *[]RepositoryDetails:
		printRepositoriesSimple(v)
	default:
		if apiErr, ok := rw.data.(*clientStats.APIError); ok {
			printErrorMessage(apiErr)
		} else {
			log.Warn("An unexpected data type was received and cannot be printed as a detailed error.")
		}
	}

	return nil
}

func getRepositoryCounts(repos *[]RepositoryDetails) map[string]int {
	counts := make(map[string]int)
	for _, repo := range *repos {
		counts[repo.Type]++
	}
	return counts
}

func printRepositoriesTable(t table.Writer, repos *[]RepositoryDetails) {
	if len(*repos) == 0 {
		t.AppendRow(table.Row{"No Repositories Available"})
		return
	}
	counts := getRepositoryCounts(repos)
	t.AppendHeader(table.Row{"Repository Type", "Count"})
	for repoType, count := range counts {
		t.AppendRow(table.Row{repoType, count})
	}
}

func printRepositoriesSimple(repos *[]RepositoryDetails) {
	log.Output("--- Repositories Summary by Type ---")
	if len(*repos) == 0 {
		log.Output("No Repositories Available")
		log.Output()
		return
	}
	counts := getRepositoryCounts(repos)
	for repoType, count := range counts {
		log.Output("- ", repoType, ": ", count)
	}
	log.Output()
}

func printReleaseBundlesTable(t table.Writer, rbResponse *ReleaseBundleResponse) {
	if len(rbResponse.ReleaseBundles) == 0 {
		t.AppendRow(table.Row{"No Release Bundles Available"})
		return
	}
	t.AppendHeader(table.Row{"Release Bundle Name", "Project Key", "Repository Key"})
	for _, rb := range rbResponse.ReleaseBundles {
		t.AppendRow(table.Row{rb.ReleaseBundleName, rb.ProjectKey, rb.RepositoryKey})
	}
}

func printReleaseBundlesSimple(rbResponse *ReleaseBundleResponse) {
	log.Output("--- Available Release Bundles ---")
	if len(rbResponse.ReleaseBundles) == 0 {
		log.Output("No Release Bundles Available")
		log.Output()
		return
	}
	for index, rb := range rbResponse.ReleaseBundles {
		log.Output("ReleaseBundle: ", index+1)
		log.Output("ReleaseBundleName: ", rb.ReleaseBundleName)
		log.Output("RepositoryKey: ", rb.RepositoryKey)
		log.Output("ProjectKey:", rb.ProjectKey)
		log.Output()
	}
}

func printArtifactoryStats(stats *ArtifactoryInfo) {
	log.Output("--- Artifactory Statistics Summary ---")
	log.Output("Total No of Binaries: ", stats.BinariesSummary.BinariesCount)
	log.Output("Total Binaries Size: ", stats.BinariesSummary.BinariesSize)
	log.Output("Total No of Artifacts: ", stats.BinariesSummary.ArtifactsCount)
	log.Output("Total Artifacts Size: ", stats.BinariesSummary.ArtifactsSize)
	log.Output("Storage Type: ", stats.FileStoreSummary.StorageType)
	log.Output()
}

func printXrayPoliciesStats(policies *[]XrayPolicy) {
	log.Output("--- Xray Policies ---")
	if len(*policies) == 0 {
		log.Output("No Xray Policies Available")
		log.Output()
		return
	}
	for index, policy := range *policies {
		log.Output("Policy: ", index+1)
		log.Output("Name: ", policy.Name)
		log.Output("Type: ", policy.Type)
		log.Output("Author: ", policy.Author)
		log.Output("Created: ", policy.Created)
		log.Output("Modified: ", policy.Modified)
		log.Output()
	}
}

func printXrayWatchesStats(watches *[]XrayWatch) {
	log.Output("--- Enforced Xray Watches ---")
	if len(*watches) == 0 {
		log.Output("No Xray Watches Available")
		log.Output()
		return
	}
	for _, watch := range *watches {
		log.Output("Name: ", watch.GeneralData.Name)
		for _, resource := range watch.ProjectResources.Resources {
			log.Output("Name:", resource.Name)
			log.Output("Type:", resource.Type)
			log.Output("BinMgrID:", resource.BinMgrID)
		}
		log.Output("")
	}
}

func printProjectsStats(projects *[]Project) {
	log.Output("--- Available Projects ---")
	if len(*projects) == 0 {
		log.Output("No Projects Available")
		log.Output()
		return
	}
	for index, project := range *projects {
		log.Output("Project: ", index+1)
		log.Output("Name: ", project.DisplayName)
		log.Output("Key: ", project.ProjectKey)
		if project.Description != "" {
			log.Output("Description: ", project.Description)
		} else {
			log.Output("Description: NA")
		}
		log.Output()
	}
}

func printJPDsStats(jpdList *[]JPD) {
	log.Output("--- Available JPDs ---")
	if len(*jpdList) == 0 {
		log.Output("No JPDs Info Available")
		log.Output()
		return
	}
	for index, jpd := range *jpdList {
		log.Output("JPD: ", index+1)
		log.Output("Name: ", jpd.Name)
		log.Output("URL: ", jpd.URL)
		log.Output("Status: ", jpd.Status.Code)
		log.Output("Detailed Status: ", jpd.Status.Message)
		log.Output("Local: ", jpd.Local)
		log.Output()
	}
}

func printErrorMessage(apiError *clientStats.APIError) {
	log.Output("---", apiError.Product, "---")
	log.Output("StatusCode - ", apiError.StatusCode)
	log.Output("StatusText - ", apiError.StatusText)
	log.Output("Suggestion - ", apiError.Suggestion)
}

func GetArtifactoryStats(client *httpclient.HttpClient, serverDetails *config.ServerDetails, httpClientDetails httputils.HttpClientDetails) (interface{}, error) {
	body, err := clientStats.GetArtifactoryStats(client, serverDetails, httpClientDetails)
	if err != nil {
		return nil, err
	}
	var stats ArtifactoryInfo
	if err := json.Unmarshal(body, &stats); err != nil {
		return nil, fmt.Errorf("error parsing Artifactory JSON: %w", err)
	}
	return &stats, nil
}

func GetRepositoriesStats(client *httpclient.HttpClient, serverDetails *config.ServerDetails, httpClientDetails httputils.HttpClientDetails) (interface{}, error) {
	body, err := clientStats.GetRepositoriesStats(client, serverDetails, httpClientDetails)
	if err != nil {
		return nil, err
	}
	var repos []RepositoryDetails
	if err := json.Unmarshal(body, &repos); err != nil {
		return nil, fmt.Errorf("error parsing repositories JSON: %w", err)
	}
	return &repos, nil
}

func GetXrayPolicies(client *httpclient.HttpClient, serverDetails *config.ServerDetails, httpClientDetails httputils.HttpClientDetails) (interface{}, error) {
	body, err := clientStats.GetXrayPolicies(client, serverDetails, httpClientDetails)
	if err != nil {
		return nil, err
	}
	var policies []XrayPolicy
	if err := json.Unmarshal(body, &policies); err != nil {
		return nil, fmt.Errorf("error parsing policies JSON: %w", err)
	}
	return &policies, nil
}

func GetXrayWatches(client *httpclient.HttpClient, serverDetails *config.ServerDetails, httpClientDetails httputils.HttpClientDetails) (interface{}, error) {
	body, err := clientStats.GetXrayWatches(client, serverDetails, httpClientDetails)
	if err != nil {
		return nil, err
	}
	var watches []XrayWatch
	if err := json.Unmarshal(body, &watches); err != nil {
		return nil, fmt.Errorf("error parsing Watches JSON: %w", err)
	}
	return &watches, nil
}

func GetProjectsStats(client *httpclient.HttpClient, serverDetails *config.ServerDetails, httpClientDetails httputils.HttpClientDetails) (interface{}, error) {
	body, err := clientStats.GetProjectsStats(client, serverDetails, httpClientDetails)
	if err != nil {
		return nil, err
	}
	var projects []Project
	if err := json.Unmarshal(body, &projects); err != nil {
		return nil, fmt.Errorf("error parsing Projects JSON: %w", err)
	}
	return &projects, nil
}

func GetJPDsStats(client *httpclient.HttpClient, serverDetails *config.ServerDetails, httpClientDetails httputils.HttpClientDetails) (interface{}, error) {
	body, err := clientStats.GetJPDsStats(client, serverDetails, httpClientDetails)
	if err != nil {
		return nil, err
	}
	var jpdList []JPD
	if err := json.Unmarshal(body, &jpdList); err != nil {
		return nil, fmt.Errorf("error parsing JPDs JSON: %w", err)
	}
	return &jpdList, nil
}

func GetReleaseBundlesStats(client *httpclient.HttpClient, serverDetails *config.ServerDetails, httpClientDetails httputils.HttpClientDetails) (interface{}, error) {
	body, err := clientStats.GetReleaseBundlesStats(client, serverDetails, httpClientDetails)
	if err != nil {
		return nil, err
	}
	var releaseBundles ReleaseBundleResponse
	if err := json.Unmarshal(body, &releaseBundles); err != nil {
		return nil, fmt.Errorf("error parsing ReleaseBundles JSON: %w", err)
	}
	return &releaseBundles, nil
}
