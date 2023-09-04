package nuget

import (
	"fmt"
	"github.com/jfrog/build-info-go/build/utils/dotnet/solution"
	"github.com/jfrog/build-info-go/entities"
	biutils "github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/sca"
	"github.com/jfrog/jfrog-client-go/utils/log"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"os"
	"github.com/jfrog/build-info-go/build/utils/dotnet/solution"
	"github.com/jfrog/build-info-go/entities"
	"os/exec"
)

const (
	nugetPackageTypeIdentifier = "nuget://"
)

func BuildDependencyTree() (dependencyTree []*xrayUtils.GraphNode, uniqueDeps []string, err error) {
	wd, err := os.Getwd()
	if err != nil {
		return
	}
	sol, err := solution.Load(wd, "", log.Logger)
	if err != nil {
		return
	}

	// If the project's build files don't exist this extra step is required in order to get project's dependencies
	if len(sol.GetDependenciesSources()) == 0 {
		log.Info("Assets files were not detected. Running 'dotnet restore' command")
		var tmpWd string
		tmpWd, err = createDirWithAssets(wd)
		if err != nil {
			return
		}

		defer func() {
			err = fileutils.RemoveTempDir(tmpWd)
			if err != nil {
				return
			}
		}()
		sol, err = solution.Load(tmpWd, "", log.Logger)
	}

	buildInfo, err := sol.BuildInfo("", log.Logger)
	if err != nil {
		return
	}
	dependencyTree, uniqueDeps = parseNugetDependencyTree(buildInfo)
	return
}

func createDirWithAssets(originalWd string) (tmpWd string, err error) {
	tmpWd, err = fileutils.CreateTempDir()
	if errorutils.CheckError(err) != nil {
		err = fmt.Errorf("failed during project build: %w", err)
		return
	}

	err = biutils.CopyDir(originalWd, tmpWd, true, nil)
	if err != nil {
		innerErr := fileutils.RemoveTempDir(tmpWd)
		if innerErr != nil {
			err = fmt.Errorf("failed during project build: %w, %w", err, innerErr)
		}
		err = fmt.Errorf("failed during project build: %w", err)

		return
	}

	err = runDotnetRestore(tmpWd)
	if err != nil {
		innerErr := fileutils.RemoveTempDir(tmpWd)
		if innerErr != nil {
			err = fmt.Errorf("failed during project build: %w, %w", err, innerErr)
		}
		err = fmt.Errorf("failed during project build: %w", err)
	}
	return
}

func runDotnetRestore(wd string) (err error) {
	command := exec.Command("dotnet", "restore")
	command.Dir = wd
	return command.Run()
}

func parseNugetDependencyTree(buildInfo *entities.BuildInfo) (nodes []*xrayUtils.GraphNode, allUniqueDeps []string) {
	uniqueDepsSet := datastructures.MakeSet[string]()
	for _, module := range buildInfo.Modules {
		treeMap := make(map[string][]string)
		for _, dependency := range module.Dependencies {
			dependencyId := nugetPackageTypeIdentifier + dependency.Id
			parent := nugetPackageTypeIdentifier + dependency.RequestedBy[0][0]
			if children, ok := treeMap[parent]; ok {
				treeMap[parent] = append(children, dependencyId)
			} else {
				treeMap[parent] = []string{dependencyId}
			}
		}
		dependencyTree, uniqueDeps := sca.BuildXrayDependencyTree(treeMap, nugetPackageTypeIdentifier+module.Id)
		nodes = append(nodes, dependencyTree)
		for _, uniqueDep := range uniqueDeps {
			uniqueDepsSet.Add(uniqueDep)
		}
	}
	allUniqueDeps = uniqueDepsSet.ToSlice()
	return
}
