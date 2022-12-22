package reposnapshot

import (
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

// Convert node to wrapper and back to verify conversions.
func TestConversions(t *testing.T) {
	root := createTestSnapshotTree(t)
	children, err := root.GetChildren()
	assert.NoError(t, err)
	node2 := children["2"]
	// Set fields to contain a non-empty value.
	node2.doneExploring = true
	node2.completed = true

	wrapper, err := root.convertToWrapper()
	assert.NoError(t, err)

	convertedRoot := wrapper.convertToNode()
	assert.True(t, reflect.DeepEqual(root, convertedRoot))
}
