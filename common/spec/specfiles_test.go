package spec

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCreateSpecFromBuildNameAndNumber(t *testing.T) {
	t.Run("Valid Inputs", func(t *testing.T) {
		spec, err := CreateSpecFromBuildNameAndNumber("Common-builds", "1.2.0")

		assert.NoError(t, err)
		assert.NotNil(t, spec)
		assert.Equal(t, "Common-builds/1.2.0", spec.Files[0].Build)
	})

	t.Run("Missing Build Name", func(t *testing.T) {
		spec, err := CreateSpecFromBuildNameAndNumber("", "1.2.0")

		assert.Error(t, err)
		assert.Nil(t, spec)
		assert.EqualError(t, err, "build name and build number must be provided")
	})

	t.Run("Missing Build Number", func(t *testing.T) {
		spec, err := CreateSpecFromBuildNameAndNumber("Common-builds", "")

		assert.Error(t, err)
		assert.Nil(t, spec)
		assert.EqualError(t, err, "build name and build number must be provided")
	})

	t.Run("Empty Build Name and Build Number", func(t *testing.T) {
		spec, err := CreateSpecFromBuildNameAndNumber("", "")

		assert.Error(t, err)
		assert.Nil(t, spec)
		assert.EqualError(t, err, "build name and build number must be provided")
	})
}
