package generic

import (
	"fmt"
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
		fmt.Fprint(w, "OK")
	}))
	defer ts.Close()
	responseBytes, err := new(PingCommand).SetServerDetails(&config.ServerDetails{ArtifactoryUrl: ts.URL + "/"}).Ping()
	if err != nil {
		t.Log(fmt.Sprintf("Error received from Artifactory following ping request: %s", err))
		t.Fail()
	}
	responseString := string(responseBytes)
	if responseString != "OK" {
		t.Log(fmt.Sprintf("Non 'OK' response received from Artifactory following ping request:: %s", responseString))
		t.Fail()
	}
}

func TestPingFailed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `{"error":"error"}`)
	}))
	defer ts.Close()
	_, err := new(PingCommand).SetServerDetails(&config.ServerDetails{ArtifactoryUrl: ts.URL + "/"}).Ping()
	if err == nil {
		t.Log("Expected error from artifactory")
		t.Fail()
	}
}
