package spec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToDistributionCommonParamsIncludesPriority(t *testing.T) {
	rule := DistributionRule{
		SiteName:     "edge1",
		CityName:     "Tel Aviv",
		CountryCodes: []string{"IL"},
		Priority:     "high",
	}

	params := rule.ToDistributionCommonParams()
	require.NotNil(t, params)
	assert.Equal(t, "edge1", params.SiteName)
	assert.Equal(t, "Tel Aviv", params.CityName)
	assert.Equal(t, []string{"IL"}, params.CountryCodes)
	assert.Equal(t, "high", params.Priority)
}

func TestCreateDistributionRulesFromFileParsesPriority(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.json")
	content := `{
  "distribution_rules": [
    {
      "site_name": "edge*",
      "priority": "medium"
    }
  ]
}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	rules, err := CreateDistributionRulesFromFile(path)
	require.NoError(t, err)
	require.Len(t, rules.DistributionRules, 1)
	assert.Equal(t, "edge*", rules.DistributionRules[0].SiteName)
	assert.Equal(t, "medium", rules.DistributionRules[0].Priority)
	assert.Equal(t, "medium", rules.DistributionRules[0].ToDistributionCommonParams().Priority)
}
