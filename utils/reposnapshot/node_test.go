package reposnapshot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Convert node to wrapper and back to verify conversions.
func TestConversions(t *testing.T) {
	root := createTestSnapshotTree(t)
	node2 := getChild(root, "2")
	// Set fields to contain a non-empty value.
	node2.NodeStatus = Completed

	wrapper, err := root.convertToWrapper()
	assert.NoError(t, err)

	convertedRoot := wrapper.convertToNode()
	node2converted := getChild(convertedRoot, "2")
	assert.Equal(t, ".", node2.parent.name)
	assert.Equal(t, Completed, node2converted.NodeStatus)
}

func TestCheckCompleted(t *testing.T) {
	zero, one, two := createThreeNodesTree(t)

	// Set completed and expect false
	checkCompleted(t, false, zero, one, two)

	// Mark done exploring and zero all file counts
	markDoneExploring(t, zero, one, two)
	decrementFilesCount(t, one, two, two)

	// Run check completed one all nodes from down to top
	checkCompleted(t, true, two, one, zero)
}

func TestCalculateTransferredFilesAndSize(t *testing.T) {
	zero, one, two := createThreeNodesTree(t)

	// Run calculate and expect that the total files count and size in "zero" are zero
	totalFilesCount, totalFilesSize, err := zero.CalculateTransferredFilesAndSize()
	assert.NoError(t, err)
	assert.Zero(t, totalFilesSize)
	assert.Zero(t, totalFilesCount)

	// Mark done exploring
	markDoneExploring(t, zero, one, two)

	// Zero the files count of "two"
	decrementFilesCount(t, two, two)
	checkCompleted(t, true, two)

	// Run calculate and expect that "zero" will contain the files count and size of "two"
	totalFilesCount, totalFilesSize, err = zero.CalculateTransferredFilesAndSize()
	assert.NoError(t, err)
	assert.EqualValues(t, 1, totalFilesSize)
	assert.EqualValues(t, 2, totalFilesCount)

	// Zero the file count of "one"
	decrementFilesCount(t, one)
	checkCompleted(t, true, one, zero)

	// Run calculate and expect that "zero" will contain the files count and size of "one" and "two"
	totalFilesCount, totalFilesSize, err = zero.CalculateTransferredFilesAndSize()
	assert.NoError(t, err)
	assert.EqualValues(t, 1, totalFilesSize)
	assert.EqualValues(t, 3, totalFilesCount)
}

// Create the following tree structure 0 --> 1 -- > 2
func createThreeNodesTree(t *testing.T) (zero, one, two *Node) {
	zero = createNodeBase(t, "0", 0, nil)
	one = createNodeBase(t, "1", 1, zero)
	two = createNodeBase(t, "2", 2, one)
	addChildren(zero, one)
	addChildren(one, two)
	return
}

func checkCompleted(t *testing.T, expected bool, nodes ...*Node) {
	for _, node := range nodes {
		assert.NoError(t, node.CheckCompleted())
		actual, err := node.IsCompleted()
		assert.NoError(t, err)
		assert.Equal(t, expected, actual)
	}
}

func markDoneExploring(t *testing.T, nodes ...*Node) {
	for _, node := range nodes {
		assert.NoError(t, node.MarkDoneExploring())
	}
}

func decrementFilesCount(t *testing.T, nodes ...*Node) {
	for _, node := range nodes {
		assert.NoError(t, node.DecrementFilesCount())
	}
}
