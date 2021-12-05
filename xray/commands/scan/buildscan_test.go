package scan

import (
	"github.com/jfrog/jfrog-client-go/xray/services"
	"reflect"
	"testing"
)

func TestGetCvesField(t *testing.T) {
	summeryCve := []services.SummeryCve{{Id: "cve123", CvssV2Score: "4.0/CVSS:2.0/AV:N/AC:L/Au:S/C:N/I:N/A:P", CvssV3Score: "5.0/CVSS:3.0/AV:N/AC:L/Au:S/C:N/I:N/A:P"}}
	expectedCve := []services.Cve{{Id: "cve123", CvssV2Score: "4.0", CvssV3Score: "5.0"}}

	cve := getCvesField(summeryCve)
	if !reflect.DeepEqual(cve, expectedCve) {
		t.Errorf("Expecting: %v, Got: %v", cve, expectedCve)
	}
}

func TestGetRootComponentFromImpactPath(t *testing.T) {
	tests := []struct {
		impactedPath          string
		buildName             string
		expectedRootComponent string
	}{
		{"default/builds/myBuild/bill.jar/com/fasterxml/jackson/core", "myBuild", "bill.jar"},
		{"proj1/builds/myBuild/bill/com/fasterxml/jackson/core/", "myBuild", "bill"},
		{"default/builds/myBuild/bill.jar", "myBuild", "bill.jar"},
	}
	for _, test := range tests {
		t.Run(test.expectedRootComponent, func(t *testing.T) {
			rootComponent := getRootComponentFromImpactPath(test.impactedPath, test.buildName)
			if rootComponent != test.expectedRootComponent {
				t.Error("Failed to parse root component. The root component is", rootComponent, " but it is expected to be", test.expectedRootComponent)
			}
		})
	}

}

func TestGetComponentImpactPaths(t *testing.T) {
	tests := []struct {
		componentId                  string
		buildName                    string
		impactPaths                  []string
		expectedComponentImpactPaths [][]services.ImpactPathNode
	}{
		{"com.fasterxml.jackson.core:jackson-databind",
			"myBuild",
			[]string{
				"default/builds/myBuild/bill.jar/com/fasterxml/jackson/jackson-databind/core",
				"default/builds/myBuild/bill.zip/com/fasterxml/jackson/jackson-databind/core",
				"default/builds/myBuild/bill.tgz/com/fasterxml/jackson/jackson-web/core",
			},
			[][]services.ImpactPathNode{
				{{ComponentId: "bill.jar"}},
				{{ComponentId: "bill.zip"}},
			},
		},
		{"com.fasterxml.jackson.core:jackson-databind",
			"myBuild",
			[]string{
				"default/builds/myBuild/bill.jar/com/fasterxml/jackson/jackson-databind/core",
				"default/builds/myBuild/bill.zip/com/fasterxml/jackson/jackson-databind/core",
				"default/builds/myBuild/bill.tgz/com/fasterxml/jackson/jackson-web/core",
			},
			[][]services.ImpactPathNode{
				{{ComponentId: "bill.jar"}},
				{{ComponentId: "bill.zip"}},
			},
		},
	}

	for _, test := range tests {
		t.Run("test", func(t *testing.T) {
			componentImpactPaths := getComponentImpactPaths(test.componentId, test.buildName, test.impactPaths)
			if !reflect.DeepEqual(componentImpactPaths, test.expectedComponentImpactPaths) {
				t.Errorf("Expecting: %v, Got: %v", componentImpactPaths, test.expectedComponentImpactPaths)
			}
		})
	}

}
