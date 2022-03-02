package audit

import (
	"os"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit"
	_go "github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/go"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/java"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/npm"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/python"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

func GenericAudit(xrayGraphScanPrams services.XrayGraphScanParams, serverDetails *config.ServerDetails, excludeTestDeps, useWrapper, insecureTls bool, args []string) (results []services.ScanResponse, err error) {
	wd, err := os.Getwd()
	if errorutils.CheckError(err) != nil {
		return
	}
	detectedTechnologies, err := coreutils.DetectTechnologies(wd, false, false)
	if err != nil {
		return
	}
	detectedTechnologiesString := coreutils.DetectedTechnologiesToString(detectedTechnologies)
	if detectedTechnologiesString == "" {
		log.Info("Could not determine the package manager / build tool used by this project.")
		return
	}
	log.Info("Detected: " + detectedTechnologiesString)
	//var failBuildErr error
	for tech := range detectedTechnologies {
		switch tech {
		case coreutils.Maven:
			results, err = AuditMvn(xrayGraphScanPrams, serverDetails, insecureTls)
		case coreutils.Gradle:
			results, err = AuditGradle(xrayGraphScanPrams, serverDetails, excludeTestDeps, useWrapper)
		case coreutils.Npm:
			results, err = AuditNpm(xrayGraphScanPrams, serverDetails, args)
		case coreutils.Go:
			results, err = AuditGo(xrayGraphScanPrams, serverDetails)
		case coreutils.Pip:
			results, err = AuditPip(xrayGraphScanPrams, serverDetails)
		case coreutils.Pipenv:
			results, err = AuditPipenv(xrayGraphScanPrams, serverDetails)
		case coreutils.Dotnet:
			break
		case coreutils.Nuget:
			results, err = AuditNuget(xrayGraphScanPrams, serverDetails)
		default:
			log.Info(string(tech), " is currently not supported")
		}
	}
	return

}

func AuditMvn(xrayGraphScanPrams services.XrayGraphScanParams, serverDetails *config.ServerDetails, insecureTls bool) (results []services.ScanResponse, err error) {
	graph, err := java.BuildMvnDependencyTree(insecureTls)
	if err != nil {
		return
	}
	return audit.Scan(graph, xrayGraphScanPrams, serverDetails)
}

func AuditGradle(xrayGraphScanPrams services.XrayGraphScanParams, serverDetails *config.ServerDetails, excludeTestDeps, useWrapper bool) (results []services.ScanResponse, err error) {
	graph, err := java.BuildGradleDependencyTree(excludeTestDeps, useWrapper)
	if err != nil {
		return
	}
	return audit.Scan(graph, xrayGraphScanPrams, serverDetails)
}

func AuditNpm(xrayGraphScanPrams services.XrayGraphScanParams, serverDetails *config.ServerDetails, args []string) (results []services.ScanResponse, err error) {
	graph, err := npm.BuildNpmDependencyTree(args)
	if err != nil {
		return
	}
	return audit.Scan([]*services.GraphNode{graph}, xrayGraphScanPrams, serverDetails)
}

func AuditGo(xrayGraphScanPrams services.XrayGraphScanParams, serverDetails *config.ServerDetails) (results []services.ScanResponse, err error) {
	graph, err := _go.BuildGoDependencyTree()
	if err != nil {
		return
	}
	return audit.Scan([]*services.GraphNode{graph}, xrayGraphScanPrams, serverDetails)
}

func AuditPip(xrayGraphScanPrams services.XrayGraphScanParams, serverDetails *config.ServerDetails) (results []services.ScanResponse, err error) {
	graph, err := python.BuildPipDependencyTree()
	if err != nil {
		return
	}
	return audit.Scan(graph, xrayGraphScanPrams, serverDetails)
}

func AuditPipenv(xrayGraphScanPrams services.XrayGraphScanParams, serverDetails *config.ServerDetails) (results []services.ScanResponse, err error) {
	graph, err := python.BuildPipenvDependencyTree()
	if err != nil {
		return
	}
	return audit.Scan([]*services.GraphNode{graph}, xrayGraphScanPrams, serverDetails)
}

func AuditNuget(xrayGraphScanPrams services.XrayGraphScanParams, serverDetails *config.ServerDetails) (results []services.ScanResponse, err error) {
	return
}
