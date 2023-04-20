package curation

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	utils2 "github.com/jfrog/jfrog-cli-core/v2/xray/commands/utils"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services/utils"
	"net/http"
	"strings"
)

type treeAnalyzer struct {
	artiManager       artifactory.ArtifactoryServicesManager
	artAuth           auth.ServiceDetails
	httpClientDetails httputils.HttpClientDetails
	url               string
	repo              string
	tech              coreutils.Technology
}

func (nc *treeAnalyzer) recursiveNodeCuration(graph *utils.GraphNode, respStatus *[]PackageStatus, parent string, isRoot bool) error {
	if parent == "" && !isRoot {
		_, name, version := getUrlNameAndVersionByTech(nc.tech, graph.Id, nc.url, nc.repo)
		parent = fmt.Sprintf("%s:%s", name, version)
	}
	for _, node := range graph.Nodes {
		packageUrl, name, version := getUrlNameAndVersionByTech(nc.tech, node.Id, nc.url, nc.repo)
		resp, _, err := nc.artiManager.Client().SendHead(packageUrl, &nc.httpClientDetails)
		if err != nil || resp.StatusCode >= 400 {
			if resp == nil {
				return err
			}
			if resp.StatusCode != http.StatusForbidden {
				log.Error(fmt.Sprintf("Failed to head package %s for download url: %s, status-code: %v, err: %v", node.Id, packageUrl, resp.StatusCode, err))
				continue
			}
		}
		if resp.StatusCode == http.StatusForbidden {
			err = nc.tryToAddBlockedPackage(packageUrl, parent, respStatus, name, version)
			if err != nil {
				log.Error(fmt.Sprintf("%v", err))
				continue
			}
		}

		if err := nc.recursiveNodeCuration(node, respStatus, parent, false); err != nil {
			return err
		}
	}
	return nil
}

func (nc *treeAnalyzer) tryToAddBlockedPackage(packageUrl string, parent string, respStatus *[]PackageStatus, name string, version string) error {
	getResp, respBody, _, err := nc.artiManager.Client().SendGet(packageUrl, true, &nc.httpClientDetails)
	if err != nil {
		if getResp == nil {
			return err
		}
		if getResp.StatusCode != http.StatusForbidden {
			log.Error(fmt.Sprintf("Failed to get package %s, version: %s for download url: %s. err: %v", name, version, packageUrl, err))
			return nil
		}
	}
	if getResp.StatusCode == http.StatusForbidden {
		respError := &utils2.ErrorsResp{}
		if err := json.Unmarshal(respBody, respError); err != nil {
			log.Error(fmt.Sprintf("failed to decode response for 'forbidden' response, err: %v", err))
			return nil
		}
		if len(respError.Errors) == 0 {
			log.Error(fmt.Sprintf("No errors messages for package %s, version: %s for download url: %s ", name, version, packageUrl))
			return nil
		}
		if strings.Contains(strings.ToLower(respError.Errors[0].Message), "jfrog packages curation") {
			depRelation := "direct"
			if parent != "" {
				depRelation = "indirect"
			}
			policies := extractPoliciesFromMsg(respError)
			*respStatus = append(*respStatus, PackageStatus{
				PackageName:    name,
				PackageVersion: version,
				Status:         Blocked,
				Policy:         policies,
				Parent:         parent,
				DepRelation:    depRelation,
				Resolved:       packageUrl,
				PkgType:        string(nc.tech),
			})
		}
	}
	return nil
}

// / message structure: Package %s:%s download was blocked by JFrog Packages Curation service due to the following policies violated {%s, %s},{%s, %s}.
func extractPoliciesFromMsg(respError *utils2.ErrorsResp) []policy {
	var policies []policy
	msg := respError.Errors[0].Message
	start := strings.Index(msg, "{")
	end := strings.Index(msg, "}")
	for end != -1 {
		exp := msg[start:end]
		exp = strings.TrimPrefix(exp, "{")
		polCond := strings.Split(exp, ",")
		if len(polCond) == 2 {
			pol := polCond[0]
			cond := polCond[1]
			policies = append(policies, policy{Policy: strings.TrimSpace(pol), Condition: strings.TrimSpace(cond)})
		}
		if len(msg) <= end+1 {
			break
		}
		msg = msg[end+1:]
		start = strings.Index(msg, "{")
		end = strings.Index(msg, "}")
	}
	return policies
}

func getUrlNameAndVersionByTech(tech coreutils.Technology, nodeId, artiUrl, repo string) (downloadUrl string, name string, version string) {
	switch tech {
	case coreutils.Npm:
		return getNameScopeAndVersion(nodeId, artiUrl, repo)
	}
	return
}

func getNameScopeAndVersion(id, artiUrl, repo string) (downloadUrl, name, version string) {
	id = strings.TrimPrefix(id, "npm://")

	nameVersion := strings.Split(id, ":")
	name = nameVersion[0]
	if len(nameVersion) > 1 {
		version = nameVersion[1]
	}
	scopeSplit := strings.Split(name, "/")
	var scope string
	if len(scopeSplit) > 1 {
		scope = scopeSplit[0]
		name = scopeSplit[1]
	}
	return buildNpmDownloadUrl(artiUrl, repo, name, scope, version), name, version
}

func buildNpmDownloadUrl(url, repo, name, scope, version string) string {
	var packageUrl string
	if scope != "" {
		packageUrl = fmt.Sprintf("%s/api/npm/%s/%s/%s/-/%s-%s.tgz", strings.TrimSuffix(url, "/"), repo, scope, name, name, version)
	} else {
		packageUrl = fmt.Sprintf("%s/api/npm/%s/%s/-/%s-%s.tgz", strings.TrimSuffix(url, "/"), repo, name, name, version)
	}
	return packageUrl
}
