package nuget

import (
	"errors"
	"fmt"
	"github.com/jfrog/build-info-go/build/utils/dotnet/solution"
	"github.com/jfrog/build-info-go/entities"
	biutils "github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/sca"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"os"
	"os/exec"
	"strings"
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

	// In case the project's dependencies sources can't be found we run 'dotnet restore' on a copy of the project in order to get its dependencies
	if !sol.DependenciesSourcesExist() {
		log.Info("Dependencies sources were not detected. Running 'dotnet restore' command")
		sol, err = runDotnetRestoreAndLoadSolution(wd)
		if err != nil {
			return
		}
	}

	buildInfo, err := sol.BuildInfo("", log.Logger)
	if err != nil {
		return
	}
	dependencyTree, uniqueDeps = parseNugetDependencyTree(buildInfo)
	return
}

func runDotnetRestore(wd string) (err error) {
	command := exec.Command("dotnet", "restore")
	command.Dir = wd
	output, err := command.CombinedOutput()
	if err != nil {
		err = errorutils.CheckErrorf("%q command failed: %s - %s", strings.Join(command.Args, " "), err.Error(), output)
	}
	return
}

func runDotnetRestoreAndLoadSolution(originalWd string) (sol solution.Solution, err error) {
	var tmpWd string
	tmpWd, err = fileutils.CreateTempDir()
	if err != nil {
		err = fmt.Errorf("failed creating temporary dir: %w", err)
		return
	}
	defer func() {
		err = errors.Join(err, fileutils.RemoveTempDir(tmpWd))
	}()

	err = biutils.CopyDir(originalWd, tmpWd, true, nil)
	if err != nil {
		err = fmt.Errorf("failed copying project to temp dir: %w", err)
		return
	}

	err = runDotnetRestore(tmpWd)
	if err != nil {
		return
	}
	sol, err = solution.Load(tmpWd, "", log.Logger)
	return
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
