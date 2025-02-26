package spec

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCreateSpecFromBuildNameAndNumber(t *testing.T) {
	t.Run("Valid Inputs", func(t *testing.T) {
		spec, err := CreateSpecFromBuildNameNumberAndProject("Common-builds", "1.2.0", "test")

		assert.NoError(t, err)
		assert.NotNil(t, spec)
		assert.Equal(t, "Common-builds/1.2.0", spec.Files[0].Build)
		assert.Equal(t, "test", spec.Files[0].Project)
	})

	t.Run("Missing Build Name", func(t *testing.T) {
		spec, err := CreateSpecFromBuildNameNumberAndProject("", "1.2.0", "")

		assert.Error(t, err)
		assert.Nil(t, spec)
		assert.EqualError(t, err, "build name and build number must be provided")
	})

	t.Run("Missing Project Name", func(t *testing.T) {
		spec, err := CreateSpecFromBuildNameNumberAndProject("Common-builds", "1.2.0", "")

		assert.NoError(t, err)
		assert.Empty(t, spec.Files[0].Project)
	})

	t.Run("Missing Build Number", func(t *testing.T) {
		spec, err := CreateSpecFromBuildNameNumberAndProject("Common-builds", "", "")

		assert.Error(t, err)
		assert.Nil(t, spec)
		assert.EqualError(t, err, "build name and build number must be provided")
	})

	t.Run("Empty Build Name and Build Number", func(t *testing.T) {
		spec, err := CreateSpecFromBuildNameNumberAndProject("", "", "")

		assert.Error(t, err)
		assert.Nil(t, spec)
		assert.EqualError(t, err, "build name and build number must be provided")
	})
}
