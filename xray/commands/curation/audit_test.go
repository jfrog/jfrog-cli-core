package curation

import (
	"fmt"
	rtUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	tests2 "github.com/jfrog/jfrog-cli-core/v2/common/tests"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"

	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
)

func Test_extractPoliciesFromMsg(t *testing.T) {
	var err error
	extractPoliciesRegex, err = regexp.Compile(extractPoliciesRegexTemplate)
	require.NoError(t, err)
	tests := []struct {
		name    string
		errResp *ErrorsResp
		expect  []Policy
	}{
		{
			name: "one policy",
			errResp: &ErrorsResp{
				Errors: []ErrorResp{
					{
						Status:  403,
						Message: "Package test:1.0.0 download was blocked by JFrog Packages Curation service due to the following policies violated {policy1, condition1}.",
					},
				},
			},
			expect: []Policy{
				{
					Policy:    "policy1",
					Condition: "condition1",
				},
			},
		},
		{
			name: "one policy",
			errResp: &ErrorsResp{
				Errors: []ErrorResp{
					{
						Status:  403,
						Message: "Package test:1.0.0 download was blocked by JFrog Packages Curation service due to the following policies violated {policy1, condition1}.",
					},
				},
			},
			expect: []Policy{
				{
					Policy:    "policy1",
					Condition: "condition1",
				},
			},
		},
		{
			name: "two policies",
			errResp: &ErrorsResp{
				Errors: []ErrorResp{
					{
						Status: 403,
						Message: "Package test:1.0.0 download was blocked by JFrog Packages Curation service due to" +
							" the following policies violated {policy1, condition1}, {policy2, condition2}.",
					},
				},
			},
			expect: []Policy{
				{
					Policy:    "policy1",
					Condition: "condition1",
				},
				{
					Policy:    "policy2",
					Condition: "condition2",
				},
			},
		},
		{
			name: "no policies",
			errResp: &ErrorsResp{
				Errors: []ErrorResp{
					{
						Status:  403,
						Message: "not the expected message format.",
					},
				},
			},
			expect: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPoliciesFromMsg(tt.errResp)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func Test_getNameScopeAndVersion(t *testing.T) {
	tests := []struct {
		name            string
		componentId     string
		artiUrl         string
		repo            string
		tech            string
		wantDownloadUrl string
		wantName        string
		wantVersion     string
	}{
		{
			name:            "npm component",
			componentId:     "npm://test:1.0.0",
			artiUrl:         "http://localhost:8000/artifactory",
			repo:            "npm",
			tech:            coreutils.Npm.ToString(),
			wantDownloadUrl: "http://localhost:8000/artifactory/api/npm/npm/test/-/test-1.0.0.tgz",
			wantName:        "test",
			wantVersion:     "1.0.0",
		},
		{
			name:            "npm component with scope",
			componentId:     "npm://dev/test:1.0.0",
			artiUrl:         "http://localhost:8000/artifactory",
			repo:            "npm",
			tech:            coreutils.Npm.ToString(),
			wantDownloadUrl: "http://localhost:8000/artifactory/api/npm/npm/dev/test/-/test-1.0.0.tgz",
			wantName:        "test",
			wantVersion:     "1.0.0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDownloadUrl, gotName, gotVersion := getNameScopeAndVersion(tt.componentId, tt.artiUrl, tt.repo, tt.repo)
			if gotDownloadUrl != tt.wantDownloadUrl {
				t.Errorf("getNameScopeAndVersion() gotDownloadUrl = %v, want %v", gotDownloadUrl, tt.wantDownloadUrl)
			}
			if gotName != tt.wantName {
				t.Errorf("getNameScopeAndVersion() gotName = %v, want %v", gotName, tt.wantName)
			}
			if gotVersion != tt.wantVersion {
				t.Errorf("getNameScopeAndVersion() gotVersion = %v, want %v", gotVersion, tt.wantVersion)
			}
		})
	}
}

func Test_treeAnalyzer_fillGraphRelations(t *testing.T) {
	tests := []struct {
		name                   string
		givenGraph             *xrayUtils.GraphNode
		givenMap               []*PackageStatus
		expectedPackagesStatus *[]*PackageStatus
	}{
		{
			name: "block indirect",
			givenGraph: &xrayUtils.GraphNode{
				Id:          "npm://root-test",
				Sha256:      "",
				Sha1:        "",
				Path:        "",
				DownloadUrl: "",
				Licenses:    nil,
				Properties:  nil,
				Nodes: []*xrayUtils.GraphNode{
					{
						Id: "npm://test-parent:1.0.0",
						Nodes: []*xrayUtils.GraphNode{
							{Id: "npm://test-child:2.0.0"},
						},
					},
				},
				OtherComponentIds: nil,
				Parent:            nil,
			},
			givenMap: []*PackageStatus{
				{
					Action:            "blocked",
					BlockedPackageUrl: "http://localhost:8046/artifactory/api/npm/npm-repo/test-child/-/test-child-2.0.0.tgz",
					PackageName:       "test-child",
					PackageVersion:    "2.0.0",
					BlockingReason:    "Policy violations",
					PkgType:           "npm",
					Policy: []Policy{
						{
							Policy:    "policy1",
							Condition: "condition1",
						},
					},
				},
			},
			expectedPackagesStatus: &[]*PackageStatus{
				{
					Action:            "blocked",
					BlockedPackageUrl: "http://localhost:8046/artifactory/api/npm/npm-repo/test-child/-/test-child-2.0.0.tgz",
					PackageName:       "test-child",
					PackageVersion:    "2.0.0",
					BlockingReason:    "Policy violations",
					PkgType:           "npm",
					Policy: []Policy{
						{
							Policy:    "policy1",
							Condition: "condition1",
						},
					},
					ParentName:    "test-parent",
					ParentVersion: "1.0.0",
					DepRelation:   "indirect",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nc := &treeAnalyzer{
				url:  "http://localhost:8046/artifactory",
				repo: "npm-repo",
				tech: "npm",
			}
			packageStatus := &[]*PackageStatus{}
			preProcessedMap := fillSyncedMap(tt.givenMap)
			nc.fillGraphRelations(tt.givenGraph, preProcessedMap, packageStatus, "", "", true)
			assert.Equal(t, *tt.expectedPackagesStatus, *packageStatus)
		})
	}
}

func fillSyncedMap(pkgStatus []*PackageStatus) *sync.Map {
	syncMap := sync.Map{}
	for _, value := range pkgStatus {
		syncMap.Store(value.BlockedPackageUrl, value)
	}
	return &syncMap
}

func Test_treeAnalyzer_getNodesStatusInParallel(t *testing.T) {
	tests := []struct {
		name            string
		expectedRequest map[string]bool
		requestToFail   map[string]bool
		givenGraph      *xrayUtils.GraphNode
		expectedResp    []*PackageStatus
		requestToError  map[string]bool
		expectedError   string
	}{
		{
			name: "npm tree - two blocked package ",
			expectedRequest: map[string]bool{
				"/api/npm/npms/json/-/json-9.0.6.tgz":      false,
				"/api/npm/npms/xml/-/xml-1.0.1.tgz":        false,
				"/api/npm/npms/lodash/-/lodash-2.20.0.tgz": false,
			},
			requestToFail: map[string]bool{
				"/api/npm/npms/xml/-/xml-1.0.1.tgz":        false,
				"/api/npm/npms/lodash/-/lodash-2.20.0.tgz": false,
			},
			givenGraph: &xrayUtils.GraphNode{
				Nodes: []*xrayUtils.GraphNode{
					{
						Id: "npm://root-test:1.0.0",
					},
					{
						Id: "npm://json:9.0.6",
					},
					{
						Id: "npm://xml:1.0.1",
					},
					{
						Id: "npm://lodash:2.20.0",
					},
				},
				OtherComponentIds: nil,
				Parent:            nil,
			},
			expectedResp: []*PackageStatus{
				{
					Action:            "blocked",
					BlockedPackageUrl: "api/npm/npms/xml/-/xml-1.0.1.tgz",
					PackageName:       "xml",
					PackageVersion:    "1.0.1",
					BlockingReason:    "Policy violations",
					PkgType:           "npm",
					Policy: []Policy{
						{
							Policy:    "pol1",
							Condition: "cond1",
						},
					},
				},
				{
					Action:            "blocked",
					BlockedPackageUrl: "api/npm/npms/lodash/-/lodash-2.20.0.tgz",
					PackageName:       "lodash",
					PackageVersion:    "2.20.0",
					BlockingReason:    "Policy violations",
					PkgType:           "npm",
					Policy: []Policy{
						{
							Policy:    "pol1",
							Condition: "cond1",
						},
					},
				},
			},
		},
		{
			name: "npm tree - two blocked one error ",
			expectedRequest: map[string]bool{
				"/api/npm/npms/json/-/json-9.0.6.tgz":      false,
				"/api/npm/npms/xml/-/xml-1.0.1.tgz":        false,
				"/api/npm/npms/lodash/-/lodash-2.20.0.tgz": false,
			},
			requestToFail: map[string]bool{
				"/api/npm/npms/xml/-/xml-1.0.1.tgz":        false,
				"/api/npm/npms/lodash/-/lodash-2.20.0.tgz": false,
			},
			requestToError: map[string]bool{
				"/api/npm/npms/json/-/json-9.0.6.tgz": false,
			},
			givenGraph: &xrayUtils.GraphNode{
				Nodes: []*xrayUtils.GraphNode{
					{
						Id: "npm://root-test:1.0.0",
					},
					{
						Id: "npm://json:9.0.6",
					},
					{
						Id: "npm://xml:1.0.1",
					},
					{
						Id: "npm://lodash:2.20.0",
					},
				},
				OtherComponentIds: nil,
				Parent:            nil,
			},
			expectedResp: []*PackageStatus{
				{
					Action:            "blocked",
					BlockedPackageUrl: "api/npm/npms/xml/-/xml-1.0.1.tgz",
					PackageName:       "xml",
					PackageVersion:    "1.0.1",
					BlockingReason:    "Policy violations",
					PkgType:           "npm",
					Policy: []Policy{
						{
							Policy:    "pol1",
							Condition: "cond1",
						},
					},
				},
				{
					Action:            "blocked",
					BlockedPackageUrl: "api/npm/npms/lodash/-/lodash-2.20.0.tgz",
					PackageName:       "lodash",
					PackageVersion:    "2.20.0",
					BlockingReason:    "Policy violations",
					PkgType:           "npm",
					Policy: []Policy{
						{
							Policy:    "pol1",
							Condition: "cond1",
						},
					},
				},
			},
			expectedError: fmt.Sprintf("failed sending HEAD to %s for package %s. Status-code: %v",
				"/api/npm/npms/json/-/json-9.0.6.tgz", "npm://json:9.0.6", http.StatusInternalServerError),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockServer, config := curationServer(t, tt.expectedRequest, tt.requestToFail, tt.requestToError)
			rtManager, err := rtUtils.CreateServiceManager(config, 2, 0, false)
			require.NoError(t, err)
			rtAuth, err := config.CreateArtAuthConfig()
			defer mockServer.Close()
			nc := &treeAnalyzer{
				rtManager:         rtManager,
				rtAuth:            rtAuth,
				httpClientDetails: rtAuth.CreateHttpClientDetails(),
				url:               rtAuth.GetUrl(),
				repo:              "npms",
				tech:              "npm",
				parallelRequests:  3,
			}
			syncMap := sync.Map{}
			if tt.requestToError == nil {
				require.NoError(t, nc.getNodesStatusInParallel(tt.givenGraph, &syncMap, "npm://test:1.0.0"))
			} else {
				gotError := nc.getNodesStatusInParallel(tt.givenGraph, &syncMap, "npm://test:1.0.0")
				require.Error(t, gotError)
				errMsgExpected := tt.expectedError[:strings.Index(tt.expectedError, "/")] + rtAuth.GetUrl() +
					tt.expectedError[strings.Index(tt.expectedError, "/")+1:]
				assert.Equal(t, errMsgExpected, gotError.Error())
			}
			for _, pkgStatus := range tt.expectedResp {
				blockedPkgUrl := fmt.Sprintf("%s%s", rtAuth.GetUrl(), pkgStatus.BlockedPackageUrl)
				value, exist := syncMap.Load(blockedPkgUrl)
				require.True(t, exist)
				gotPkgStatusValue := value.(*PackageStatus)
				pkgStatus.BlockedPackageUrl = blockedPkgUrl
				assert.Equal(t, gotPkgStatusValue, pkgStatus)
			}
			for _, requestDone := range tt.expectedRequest {
				assert.True(t, requestDone)
			}
		})
	}
}

func curationServer(t *testing.T, expectedRequest map[string]bool, requestToFail map[string]bool, requestToError map[string]bool) (*httptest.Server, *config.ServerDetails) {
	serverMock, config, _ := tests2.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			if _, exist := expectedRequest[r.RequestURI]; exist {
				expectedRequest[r.RequestURI] = true
			}
			if _, exist := requestToFail[r.RequestURI]; exist {
				w.WriteHeader(http.StatusForbidden)
			}
			if _, exist := requestToError[r.RequestURI]; exist {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}
		if r.Method == http.MethodGet {
			if _, exist := requestToFail[r.RequestURI]; exist {
				w.WriteHeader(http.StatusForbidden)
				_, err := w.Write([]byte("{\n    \"errors\": [\n        {\n            \"status\": 403,\n            " +
					"\"message\": \"Package download was blocked by JFrog Packages " +
					"Curation service due to the following policies violated {pol1, cond1}\"\n        }\n    ]\n}"))
				require.NoError(t, err)
			}
		}
	})
	return serverMock, config
}
