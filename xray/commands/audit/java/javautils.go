package java

import (
	"strconv"
	"time"

	buildinfo "github.com/jfrog/build-info-go/entities"

	artifactoryUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

const (
	GavPackageTypeIdentifier = "gav://"
)

func createBuildConfiguration(buildName string) (*artifactoryUtils.BuildConfiguration, func(err error)) {
	buildConfiguration := artifactoryUtils.NewBuildConfiguration(buildName, strconv.FormatInt(time.Now().Unix(), 10), "", "")
	return buildConfiguration, func(err error) {
		buildName, err := buildConfiguration.GetBuildName()
		if err != nil {
			return
		}
		buildNumber, err := buildConfiguration.GetBuildNumber()
		if err != nil {
			return
		}
		err = artifactoryUtils.RemoveBuildDir(buildName, buildNumber, buildConfiguration.GetProject())
	}
}

// Create a dependency tree for each one of the modules in the build.
// buildName - audit-mvn or audit-gradle
func createGavDependencyTree(buildConfig *artifactoryUtils.BuildConfiguration) ([]*services.GraphNode, error) {
	buildName, err := buildConfig.GetBuildName()
	if err != nil {
		return nil, err
	}
	buildNumber, err := buildConfig.GetBuildNumber()
	if err != nil {
		return nil, err
	}
	generatedBuildsInfos, err := artifactoryUtils.GetGeneratedBuildsInfo(buildName, buildNumber, buildConfig.GetProject())
	if err != nil {
		return nil, err
	}
	if len(generatedBuildsInfos) == 0 {
		return nil, errorutils.CheckErrorf("Couldn't find build " + buildName + "/" + buildNumber)
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
	for index, dependency := range module.Dependencies {
		requestedBy := dependency.RequestedBy
		if isDirectDependency(module.Id, requestedBy) {
			// If no parents at all or the direct parent is the module, assume dependency is a direct
			directDependencies[dependency.Id] = dependency
			continue
		}

		for _, parent := range requestedBy {
			// we use '&module.Dependencies[index]' to avoid reusing the &dependency pointer
			parentToChildren.putChild(GavPackageTypeIdentifier+parent[0], &module.Dependencies[index])
		}
	}

	for _, directDependency := range directDependencies {
		populateTransitiveDependencies(moduleTree, directDependency.Id, parentToChildren, []string{})
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

func populateTransitiveDependencies(parent *services.GraphNode, dependencyId string, parentToChildren *dependencyMultimap, idsAdded []string) {
	if hasLoop(idsAdded, dependencyId) {
		return
	}
	idsAdded = append(idsAdded, dependencyId)
	node := &services.GraphNode{
		Id:    GavPackageTypeIdentifier + dependencyId,
		Nodes: []*services.GraphNode{},
	}
	parent.Nodes = append(parent.Nodes, node)
	for _, child := range parentToChildren.getChildren(node.Id) {
		populateTransitiveDependencies(node, child.Id, parentToChildren, idsAdded)
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
