package audit

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands"
	xrutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"strings"
)

func RunScanGraph(modulesDependencyTrees []*services.GraphNode, serverDetails *config.ServerDetails, includeVulnerabilities bool, includeLicenses bool, targetRepoPath, projectKey string, watches []string, outputFormat OutputFormat) error {
	xrayManager, err := commands.CreateXrayServiceManager(serverDetails)
	if err != nil {
		return err
	}
	var results []services.ScanResponse
	for _, moduleDependencyTree := range modulesDependencyTrees {
		params := &services.XrayGraphScanParams{
			Graph:      moduleDependencyTree,
			RepoPath:   targetRepoPath,
			Watches:    watches,
			ProjectKey: projectKey,
		}

		// Print the module ID
		log.Info("Scanning module " + moduleDependencyTree.Id[strings.Index(moduleDependencyTree.Id, "//")+2:] + "...")

		// Scan and wait for results
		scanId, err := xrayManager.ScanGraph(*params)
		if err != nil {
			return err
		}
		scanResults, err := xrayManager.GetScanGraphResults(scanId, includeVulnerabilities, includeLicenses)
		if err != nil {
			return err
		}
		results = append(results, *scanResults)
	}
	err = xrutils.PrintScanResults(results, outputFormat == Table, includeVulnerabilities, includeLicenses, false)
	return err
}
