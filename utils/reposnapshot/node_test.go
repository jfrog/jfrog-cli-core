package reposnapshot

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

// Convert node to wrapper and back to verify conversions.
func TestConversions(t *testing.T) {
	root := createTestSnapshotTree(t)
	node2 := getChild(root, "2")
	// Set fields to contain a non-empty value.
	node2.completed = true

	wrapper, err := root.convertToWrapper()
	assert.NoError(t, err)

	convertedRoot := wrapper.convertToNode()
	node2converted := getChild(convertedRoot, "2")
	assert.Equal(t, ".", node2.parent.name)
	assert.True(t, node2converted.completed)
}
