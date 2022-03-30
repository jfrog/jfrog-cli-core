package generic

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/log"
)

func TestPingSuccess(t *testing.T) {
	log.SetDefaultLogger()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := fmt.Fprint(w, "OK")
		assert.NoError(t, err)
	}))
	defer ts.Close()
	responseBytes, err := new(PingCommand).SetServerDetails(&config.ServerDetails{ArtifactoryUrl: ts.URL + "/"}).Ping()
	if err != nil {
		t.Logf("Error received from Artifactory following ping request: %s", err)
		t.Fail()
	}
	responseString := string(responseBytes)
	if responseString != "OK" {
		t.Logf("Non 'OK' response received from Artifactory following ping request:: %s", responseString)
		t.Fail()
	}
}

func TestPingFailed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, err := fmt.Fprint(w, `{"error":"error"}`)
		assert.NoError(t, err)
	}))
	defer ts.Close()
	_, err := new(PingCommand).SetServerDetails(&config.ServerDetails{ArtifactoryUrl: ts.URL + "/"}).Ping()
	if err == nil {
		t.Log("Expected error from artifactory")
		t.Fail()
	}
}
