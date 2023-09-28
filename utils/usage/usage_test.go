package usage

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/usage"
	ecosysusage "github.com/jfrog/jfrog-client-go/utils/usage"
	xrayusage "github.com/jfrog/jfrog-client-go/xray/usage"
	"github.com/stretchr/testify/assert"
)

const (
	productName = "test-product"
	serverUrl   = "server-url"
)

var (
	features = []ReportFeature{
		{
			FeatureId: "featureId2",
			ClientId:  "clientId",
			Attributes: []ReportUsageAttribute{
				{AttributeName: "attribute", AttributeValue: "value"},
			},
		},
		{FeatureId: "featureId", ClientId: "clientId2"},
		{FeatureId: "featureId"},
	}
	artifactoryFeatures = []usage.Feature{
		{
			FeatureId:  "featureId2",
			ClientId:   "clientId",
			Attributes: map[string]string{"attribute": "value"},
		},
		{FeatureId: "featureId", ClientId: "clientId2"},
		{FeatureId: "featureId"},
	}
	xrayEvents = []xrayusage.ReportXrayEventData{
		{
			ProductId: productName,
			EventId:   "server_" + productName + "_featureId2",
			Attributes: map[string]string{
				"clientId":  "clientId",
				"attribute": "value",
			},
			Origin: "API_CLI",
		},
		{
			ProductId:  productName,
			EventId:    "server_" + productName + "_featureId",
			Attributes: map[string]string{"clientId": "clientId2"},
			Origin:     "API_CLI",
		},
		{
			ProductId: productName,
			EventId:   "server_" + productName + "_featureId",
			Origin:    "API_CLI",
		},
	}
	ecosystemData = []ecosysusage.ReportEcosystemUsageData{
		{
			ProductId: productName,
			AccountId: serverUrl,
			ClientId:  "clientId",
			Features:  []string{"featureId2"},
		},
		{
			ProductId: productName,
			AccountId: serverUrl,
			ClientId:  "clientId2",
			Features:  []string{"featureId"},
		},
		{
			ProductId: productName,
			AccountId: serverUrl,
			Features:  []string{"featureId"},
		},
	}
)

func TestConvertToArtifactoryUsage(t *testing.T) {
	reporter := NewUsageReporter(productName, &config.ServerDetails{ArtifactoryUrl: serverUrl + "/"})
	for i := 0; i < len(features); i++ {
		assert.Equal(t, artifactoryFeatures[i], reporter.convertAttributesToArtifactoryFeatures(features[i])[0])
	}
}

func TestConvertToEcosystemUsage(t *testing.T) {
	reporter := NewUsageReporter(productName, &config.ServerDetails{Url: serverUrl})
	for i := 0; i < len(features); i++ {
		report, err := reporter.convertAttributesToEcosystemReports(features[i])
		assert.NoError(t, err)
		assert.Equal(t, ecosystemData[i], report[0])
	}
}

func TestReportArtifactoryUsage(t *testing.T) {
	const commandName = "test-command"
	server := httptest.NewServer(createArtifactoryUsageHandler(t, productName, commandName))
	defer server.Close()
	serverDetails := &config.ServerDetails{ArtifactoryUrl: server.URL + "/"}

	reporter := NewUsageReporter(productName, serverDetails).SetSendToEcosystem(false).SetSendToXray(false)

	reporter.Report(ReportFeature{
		FeatureId: commandName,
	})
	assert.NoError(t, reporter.WaitForResponses())
}

func TestReportArtifactoryUsageError(t *testing.T) {
	reporter := NewUsageReporter("", &config.ServerDetails{}).SetSendToEcosystem(false).SetSendToXray(false)
	reporter.Report(ReportFeature{
		FeatureId: "",
	})
	assert.Error(t, reporter.WaitForResponses())

	server := httptest.NewServer(create404UsageHandler(t))
	defer server.Close()
	reporter = NewUsageReporter("", &config.ServerDetails{ArtifactoryUrl: server.URL + "/"}).SetSendToEcosystem(false).SetSendToXray(false)
	reporter.Report(ReportFeature{
		FeatureId: "",
	})
	assert.Error(t, reporter.WaitForResponses())
}

func createArtifactoryUsageHandler(t *testing.T, productName, commandName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/system/version" {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{"version":"6.9.0"}`))
			assert.NoError(t, err)
			return
		}
		if r.RequestURI == "/api/system/usage" {
			// Check request
			buf := new(bytes.Buffer)
			_, err := buf.ReadFrom(r.Body)
			assert.NoError(t, err)
			assert.Equal(t, fmt.Sprintf(`{"productId":"%s","features":[{"featureId":"%s"}]}`, productName, commandName), buf.String())

			// Send response OK
			w.WriteHeader(http.StatusOK)
			_, err = w.Write([]byte("{}"))
			assert.NoError(t, err)
		}
	}
}

func TestReportEcosystemUsageError(t *testing.T) {
	// No features
	reporter := NewUsageReporter("", &config.ServerDetails{}).SetSendToArtifactory(false).SetSendToXray(false)
	reporter.Report()
	assert.Error(t, reporter.WaitForResponses())
	// Empty features
	reporter.Report(ReportFeature{
		FeatureId: "",
		ClientId:  "client",
	})
	assert.Error(t, reporter.WaitForResponses())
}

func create404UsageHandler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}
}
