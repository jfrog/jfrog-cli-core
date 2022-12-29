package utils

import (
	"net/http"
	"testing"

	commonTests "github.com/jfrog/jfrog-cli-core/v2/common/tests"
	"github.com/stretchr/testify/assert"
)

func TestIsDefaultCredentialsDefault(t *testing.T) {
	unlockCounter := 0
	testServer, _, serviceManager := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/security/lockedUsers" {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("[]"))
			assert.NoError(t, err)
		} else if r.RequestURI == "/api/system/ping" {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("OK"))
			assert.NoError(t, err)
		} else {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("User admin was successfully unlocked"))
			assert.NoError(t, err)
			unlockCounter++
		}
	})
	defer testServer.Close()

	isDefaultCreds, err := IsDefaultCredentials(serviceManager, testServer.URL)
	assert.NoError(t, err)
	assert.True(t, isDefaultCreds)
	assert.Equal(t, 0, unlockCounter)
}

func TestIsDefaultCredentialsNotDefault(t *testing.T) {
	unlockCounter := 0
	testServer, _, serviceManager := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/security/lockedUsers" {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("[]"))
			assert.NoError(t, err)
		} else if r.RequestURI == "/api/system/ping" {
			w.WriteHeader(http.StatusUnauthorized)
			_, err := w.Write([]byte("{\n  \"errors\" : [ {\n    \"status\" : 401,\n    \"message\" : \"Bad credentials\"\n  } ]\n}"))
			assert.NoError(t, err)
		} else {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("User admin was successfully unlocked"))
			assert.NoError(t, err)
			unlockCounter++
		}
	})
	defer testServer.Close()

	isDefaultCreds, err := IsDefaultCredentials(serviceManager, testServer.URL)
	assert.NoError(t, err)
	assert.False(t, isDefaultCreds)
	assert.Equal(t, 1, unlockCounter)
}

func TestIsDefaultCredentialsLocked(t *testing.T) {
	pingCounter := 0
	unlockCounter := 0
	testServer, _, serviceManager := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/security/lockedUsers" {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("[ \"admin\" ]"))
			assert.NoError(t, err)
		} else if r.RequestURI == "/api/system/ping" {
			w.WriteHeader(http.StatusUnauthorized)
			_, err := w.Write([]byte("{\n  \"errors\" : [ {\n    \"status\" : 401,\n    \"message\" : \"Bad credentials\"\n  } ]\n}"))
			assert.NoError(t, err)
			pingCounter++
		} else {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("User admin was successfully unlocked"))
			assert.NoError(t, err)
			unlockCounter++
		}
	})
	defer testServer.Close()

	isDefaultCreds, err := IsDefaultCredentials(serviceManager, testServer.URL)
	assert.NoError(t, err)
	assert.False(t, isDefaultCreds)
	assert.Equal(t, 0, pingCounter)
	assert.Equal(t, 0, unlockCounter)
}
