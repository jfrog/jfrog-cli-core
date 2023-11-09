package java

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	xrayutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
)

const (
	GavPackageTypeIdentifier = "gav://"
)

func BuildDependencyTree(params xrayutils.AuditParams, tech coreutils.Technology) ([]*xrayUtils.GraphNode, []string, error) {
	serverDetails, err := params.ServerDetails()
	if err != nil {
		return nil, nil, err
	}
	depTreeParams := &DepTreeParams{
		Tool:                  tech,
		IsMvnDepTreeInstalled: params.IsMavenDepTreeInstalled(),
		UseWrapper:            params.UseWrapper(),
		Server:                serverDetails,
		DepsRepo:              params.DepsRepo(),
	}
	if tech == coreutils.Maven {
		return buildMavenDependencyTree(depTreeParams)
	}
	return buildGradleDependencyTree(depTreeParams)
}
