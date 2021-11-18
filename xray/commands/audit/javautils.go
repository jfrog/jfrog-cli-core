package audit

import (
	buildinfo "github.com/jfrog/build-info-go/entities"
	"strconv"
	"time"

	artifactoryUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

const (
	GavPackageTypeIdentifier = "gav://"
)

func createBuildConfiguration(buildName string) (*artifactoryUtils.BuildConfiguration, func(err error)) {
	buildConfiguration := &artifactoryUtils.BuildConfiguration{
		BuildName:   buildName,
		BuildNumber: strconv.FormatInt(time.Now().Unix(), 10),
	}
	return buildConfiguration, func(err error) {
		err = artifactoryUtils.RemoveBuildDir(buildConfiguration.BuildName, buildConfiguration.BuildNumber, buildConfiguration.Project)
	}
}

// Create a dependency tree for each one of the modules in the build.
// buildName - audit-mvn or audit-gradle
func createGavDependencyTree(buildConfig *artifactoryUtils.BuildConfiguration) ([]*services.GraphNode, error) {
	generatedBuildsInfos, err := artifactoryUtils.GetGeneratedBuildsInfo(buildConfig.BuildName, buildConfig.BuildNumber, buildConfig.Project)
	if err != nil {
		return nil, err
	}
	if len(generatedBuildsInfos) == 0 {
		return nil, errorutils.CheckErrorf("Couldn't find build " + buildConfig.BuildName + "/" + buildConfig.BuildNumber)
	}
	modules := []*services.GraphNode{}
	for _, module := range generatedBuildsInfos[0].Modules {
		modules = append(modules, addModuleTree(module))
	}

	return modules, nil
}

func addModuleTree(module buildinfo.Module) *services.GraphNode {
	moduleTree := &services.GraphNode{
		Id: GavPackageTypeIdentifier + module.Id,
	}

	directDependencies := make(map[string]buildinfo.Dependency)
	parentToChildren := newDependencyMultimap()
	for _, dependency := range module.Dependencies {
		requestedBy := dependency.RequestedBy
		if isDirectDependency(module.Id, requestedBy) {
			// If no parents at all or the direct parent is the module, assume dependency is a direct
			directDependencies[dependency.Id] = dependency
			continue
		}

		for _, parent := range requestedBy {
			parentToChildren.putChild(GavPackageTypeIdentifier+parent[0], &dependency)
		}
	}

	for _, directDependency := range directDependencies {
		populateTransitiveDependencies(moduleTree, &directDependency, parentToChildren, []string{})
	}
	return moduleTree
}

func isDirectDependency(moduleId string, requestedBy [][]string) bool {
	if len(requestedBy) == 0 || len(requestedBy[0]) == 0 {
		// If no parents at all, assume dependency is direct
		return true
	}
	for _, directParent := range requestedBy {
		if directParent[0] == moduleId {
			return true
		}
	}

	return false
}

func populateTransitiveDependencies(parent *services.GraphNode, dependency *buildinfo.Dependency, parentToChildren *dependencyMultimap, idsAdded []string) {
	if hasLoop(idsAdded, dependency.Id) {
		return
	}
	idsAdded = append(idsAdded, dependency.Id)
	node := &services.GraphNode{
		Id:    GavPackageTypeIdentifier + dependency.Id,
		Nodes: []*services.GraphNode{},
	}
	parent.Nodes = append(parent.Nodes, node)
	for _, child := range parentToChildren.getChildren(node.Id) {
		populateTransitiveDependencies(node, child, parentToChildren, idsAdded)
	}
}

func hasLoop(idsAdded []string, idToAdd string) bool {
	for _, id := range idsAdded {
		if id == idToAdd {
			return true
		}
	}
	return false
}

type dependencyMultimap struct {
	multimap map[string]map[string]*buildinfo.Dependency
}

func newDependencyMultimap() *dependencyMultimap {
	dependencyMultimap := new(dependencyMultimap)
	dependencyMultimap.multimap = make(map[string]map[string]*buildinfo.Dependency)
	return dependencyMultimap
}

func (dm *dependencyMultimap) putChild(parent string, child *buildinfo.Dependency) {
	if dm.multimap[parent] == nil {
		dm.multimap[parent] = make(map[string]*buildinfo.Dependency)
	}
	dm.multimap[parent][child.Id] = child
}

func (dm *dependencyMultimap) getChildren(parent string) map[string]*buildinfo.Dependency {
	return dm.multimap[parent]
}
