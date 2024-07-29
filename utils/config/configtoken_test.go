package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	//#nosec G101 - Dummy token for tests.
	v1Token = "eyJ2ZXJzaW9uIjoxLCJ1cmwiOiJodHRwOi8vMTI3LjAuMC4xOjgwODEvYXJ0aWZhY3RvcnkvIiwiZGlzdHJpYnV0aW9uVXJsIjoiaHR0cDovLzEyNy4wLjAuMTo4MDgxL2Rpc3RyaWJ1dGlvbiIsInVzZXIiOiJhZG1pbiIsInBhc3N3b3JkIjoicGFzc3dvcmQiLCJ0b2tlblJlZnJlc2hJbnRlcnZhbCI6NjAsInNlcnZlcklkIjoibG9jYWwifQ=="
	//#nosec G101 - Dummy token for tests.
	v2Token = `eyJ2ZXJzaW9uIjoxLCJ1cmwiOiJodHRwOi8vMTI3LjAuMC4xOjgwODEvIiwiYXJ0aWZhY3RvcnlVcmwiOiJodHRwOi8vMTI3LjAuMC4xOjgwODEvYXJ0aWZhY3RvcnkvIiwiZGlzdHJpYnV0aW9uVXJsIjoiaHR0cDovLzEyNy4wLjAuMTo4MDgxL2Rpc3RyaWJ1dGlvbi8iLCJ4cmF5VXJsIjoiaHR0cDovLzEyNy4wLjAuMTo4MDgxL3hyYXkvIiwib
Wlzc2lvbkNvbnRyb2xVcmwiOiJodHRwOi8vMTI3LjAuMC4xOjgwODEvbWMvIiwicGlwZWxpbmVzVXJsIjoiaHR0cDovLzEyNy4wLjAuMTo4MDgxL3BpcGVsaW5lcy8iLCJ1c2VyIjoiYWRtaW4iLCJwYXNzd29yZCI6InBhc3N3b3JkIiwidG9rZW5SZWZyZXNoSW50ZXJ2YWwiOjYwLCJzZXJ2ZXJJZCI6ImxvY2FsIn0=`
)

func TestImportFromV1(t *testing.T) {
	serverDetails, err := Import(v1Token)
	assert.NoError(t, err)

	assert.Equal(t, "local", serverDetails.ServerId)
	assert.Empty(t, serverDetails.Url)
	assert.Equal(t, "http://127.0.0.1:8081/artifactory/", serverDetails.ArtifactoryUrl)
	assert.Equal(t, "http://127.0.0.1:8081/distribution", serverDetails.DistributionUrl)
	assert.Equal(t, "admin", serverDetails.User)
	assert.Equal(t, "password", serverDetails.Password)
}

func TestImportFromV2(t *testing.T) {
	serverDetails, err := Import(v2Token)
	assert.NoError(t, err)

	assert.Equal(t, "local", serverDetails.ServerId)
	assert.Equal(t, "http://127.0.0.1:8081/", serverDetails.Url)
	assert.Equal(t, "http://127.0.0.1:8081/artifactory/", serverDetails.ArtifactoryUrl)
	assert.Equal(t, "http://127.0.0.1:8081/distribution/", serverDetails.DistributionUrl)
	assert.Equal(t, "http://127.0.0.1:8081/xray/", serverDetails.XrayUrl)
	assert.Equal(t, "http://127.0.0.1:8081/mc/", serverDetails.MissionControlUrl)
	assert.Equal(t, "http://127.0.0.1:8081/pipelines/", serverDetails.PipelinesUrl)
	assert.Equal(t, "admin", serverDetails.User)
	assert.Equal(t, "password", serverDetails.Password)
}
