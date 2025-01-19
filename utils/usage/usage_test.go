package usage

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/usage"
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
)

func TestConvertToArtifactoryUsage(t *testing.T) {
	reporter := NewUsageReporter(productName, &config.ServerDetails{ArtifactoryUrl: serverUrl + "/"})
	for i := 0; i < len(features); i++ {
		assert.Equal(t, artifactoryFeatures[i], reporter.convertAttributesToArtifactoryFeatures(features[i])[0])
	}
}

func TestReportArtifactoryUsage(t *testing.T) {
	const commandName = "test-command"
	server := httptest.NewServer(createArtifactoryUsageHandler(t, productName, commandName))
	defer server.Close()
	serverDetails := &config.ServerDetails{ArtifactoryUrl: server.URL + "/"}

	reporter := NewUsageReporter(productName, serverDetails).SetSendToXray(false)

	reporter.Report(ReportFeature{
		FeatureId: commandName,
	})
	assert.NoError(t, reporter.WaitForResponses())
}

func TestReportArtifactoryUsageError(t *testing.T) {
	reporter := NewUsageReporter("", &config.ServerDetails{}).SetSendToXray(false)
	reporter.Report(ReportFeature{
		FeatureId: "",
	})
	assert.Error(t, reporter.WaitForResponses())

	server := httptest.NewServer(create404UsageHandler(t))
	defer server.Close()
	reporter = NewUsageReporter("", &config.ServerDetails{ArtifactoryUrl: server.URL + "/"}).SetSendToXray(false)
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

func TestReportXrayUsage(t *testing.T) {
	const productName = "test-product"
	const commandName = "test-command"
	const clientName = "test-client"

	server := httptest.NewServer(createXrayUsageHandler(t, productName, commandName, clientName))
	defer server.Close()
	serverDetails := &config.ServerDetails{XrayUrl: server.URL + "/"}

	reporter := NewUsageReporter(productName, serverDetails).SetSendToArtifactory(false)

	reporter.Report(ReportFeature{
		FeatureId: commandName,
		ClientId:  clientName,
	})
	assert.NoError(t, reporter.WaitForResponses())
}

func TestReportXrayError(t *testing.T) {
	reporter := NewUsageReporter("", &config.ServerDetails{}).SetSendToArtifactory(false)
	reporter.Report(ReportFeature{})
	assert.Error(t, reporter.WaitForResponses())

	server := httptest.NewServer(create404UsageHandler(t))
	defer server.Close()
	reporter = NewUsageReporter("", &config.ServerDetails{ArtifactoryUrl: server.URL + "/"}).SetSendToArtifactory(false)
	reporter.Report(ReportFeature{})
	assert.Error(t, reporter.WaitForResponses())
}

func createXrayUsageHandler(t *testing.T, productId, commandName, clientId string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/v1/system/version" {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{"xray_version":"6.9.0"}`))
			assert.NoError(t, err)
			return
		}
		if r.RequestURI == "/api/v1/usage/events/send" {
			// Check request
			buf := new(bytes.Buffer)
			_, err := buf.ReadFrom(r.Body)
			assert.NoError(t, err)
			featureId := xrayusage.GetExpectedXrayEventName(productId, commandName)
			assert.Equal(t, fmt.Sprintf(`[{"data":{"clientId":"%s"},"product_name":"%s","event_name":"%s","origin":"API_CLI"}]`, clientId, productId, featureId), buf.String())

			// Send response OK
			w.WriteHeader(http.StatusOK)
			_, err = w.Write([]byte("{}"))
			assert.NoError(t, err)
			return
		}
		assert.Fail(t, "Unexpected request URI", r.RequestURI)
	}
}

func create404UsageHandler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}
}
