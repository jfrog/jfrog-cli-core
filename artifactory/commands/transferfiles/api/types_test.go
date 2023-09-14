package api

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppendUploadCandidateIfNeeded(t *testing.T) {
	uploadChunk := &UploadChunk{}

	// Regular file
	uploadChunk.AppendUploadCandidateIfNeeded(FileRepresentation{Name: "regular-file"}, false)
	assert.Len(t, uploadChunk.UploadCandidates, 1)

	// Build info
	uploadChunk.AppendUploadCandidateIfNeeded(FileRepresentation{Name: "build-info.json"}, true)
	assert.Len(t, uploadChunk.UploadCandidates, 2)

	// Directory in build info - should be skipped
	uploadChunk.AppendUploadCandidateIfNeeded(FileRepresentation{}, true)
	assert.Len(t, uploadChunk.UploadCandidates, 2)
}

var isChunkFullCases = []struct {
	files  []FileRepresentation
	isFull bool
}{
	{[]FileRepresentation{}, false},
	{[]FileRepresentation{{Name: "slim-jim", Size: 10737418}}, false},
	{[]FileRepresentation{{Name: "fat-vinny", Size: 1073741825}}, true},
}

func TestIsChunkFull(t *testing.T) {
	for _, testCase := range isChunkFullCases {
		t.Run("", func(t *testing.T) {
			uploadChunk := &UploadChunk{UploadCandidates: testCase.files}
			assert.Equal(t, testCase.isFull, uploadChunk.IsChunkFull())
		})
	}
	t.Run("", func(t *testing.T) {
		uploadChunk := &UploadChunk{}
		for i := 0; i < 17; i++ {
			uploadChunk.AppendUploadCandidateIfNeeded(FileRepresentation{Name: fmt.Sprintf("%d", i)}, false)
		}
		assert.True(t, uploadChunk.IsChunkFull())
	})
}
