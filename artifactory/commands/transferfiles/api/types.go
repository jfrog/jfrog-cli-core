package api

import (
	"fmt"

	"github.com/jfrog/jfrog-client-go/utils/log"
)

type ProcessStatusType string
type ChunkFileStatusType string
type ChunkId string
type NodeId string

const (
	Done       ProcessStatusType = "DONE"
	InProgress ProcessStatusType = "IN_PROGRESS"

	Success             ChunkFileStatusType = "SUCCESS"
	Fail                ChunkFileStatusType = "FAIL"
	SkippedLargeProps   ChunkFileStatusType = "SKIPPED_LARGE_PROPS"
	SkippedMetadataFile ChunkFileStatusType = "SKIPPED_METADATA_FILE"
	SkippedNonEmptyDir  ChunkFileStatusType = "SKIPPED_NON_EMPTY_DIR"

	Phase1 int = 0
	Phase2 int = 1
	Phase3 int = 2

	maxFilesInChunk = 16
	// 1 GiB
	maxBytesInChunk = 1 << 30
)

type TargetAuth struct {
	TargetArtifactoryUrl string `json:"target_artifactory_url,omitempty"`
	TargetUsername       string `json:"target_username,omitempty"`
	TargetPassword       string `json:"target_password,omitempty"`
	TargetToken          string `json:"target_token,omitempty"`
	TargetProxyKey       string `json:"target_proxy_key,omitempty"`
}

type UploadChunk struct {
	// Authentication details of the target server
	TargetAuth
	// Files and folders to transfer
	UploadCandidates []FileRepresentation `json:"upload_candidates,omitempty"`
	// True if should check for the existence of artifacts on the target filestore
	CheckExistenceInFilestore bool `json:"check_existence_in_filestore,omitempty"`
	// True if should skip file filtering in the Data Transfer plugin
	SkipFileFiltering bool `json:"skip_file_filtering,omitempty"`
}

type FileRepresentation struct {
	Repo        string `json:"repo,omitempty"`
	Path        string `json:"path,omitempty"`
	Name        string `json:"name,omitempty"`
	Size        int64  `json:"size,omitempty"`
	NonEmptyDir bool   `json:"non_empty_dir,omitempty"`
}

type UploadChunkResponse struct {
	NodeIdResponse
	UuidTokenResponse
}

type UploadChunksStatusBody struct {
	AwaitingStatusChunks []ChunkId `json:"awaiting_status_chunks,omitempty"`
	ChunksToDelete       []ChunkId `json:"chunks_to_delete,omitempty"`
}

type UploadChunksStatusResponse struct {
	NodeIdResponse
	ChunksStatus  []ChunkStatus `json:"chunks_status,omitempty"`
	DeletedChunks []string      `json:"deleted_chunks,omitempty"`
}

type ChunkStatus struct {
	UuidTokenResponse
	Status ProcessStatusType          `json:"status,omitempty"`
	Files  []FileUploadStatusResponse `json:"files,omitempty"`
}

type FileUploadStatusResponse struct {
	FileRepresentation
	SizeBytes        int64               `json:"size_bytes,omitempty"`
	ChecksumDeployed bool                `json:"checksum_deployed,omitempty"`
	Status           ChunkFileStatusType `json:"status,omitempty"`
	StatusCode       int                 `json:"status_code,omitempty"`
	Reason           string              `json:"reason,omitempty"`
}

type NodeIdResponse struct {
	NodeId string `json:"node_id,omitempty"`
}

type UuidTokenResponse struct {
	UuidToken string `json:"uuid_token,omitempty"`
}

// Append upload candidate to the list of upload candidates. Skip empty directories in build-info repositories.
// file          - The upload candidate
// buildInfoRepo - True if this is a build-info repository
func (uc *UploadChunk) AppendUploadCandidateIfNeeded(file FileRepresentation, buildInfoRepo bool) {
	if buildInfoRepo && file.Name == "" {
		log.Debug(fmt.Sprintf("Skipping unneeded empty dir '%s' in the build-info repository '%s'", file.Path, file.Repo))
		return
	}
	uc.UploadCandidates = append(uc.UploadCandidates, file)
}

// Return true if the chunk contains at least 16 files or at least 1GiB in total
func (uc *UploadChunk) IsChunkFull() bool {
	if len(uc.UploadCandidates) >= maxFilesInChunk {
		return true
	}
	var totalSize int64 = 0
	for _, uploadCandidate := range uc.UploadCandidates {
		totalSize += uploadCandidate.Size
		if totalSize > maxBytesInChunk {
			return true
		}
	}
	return false
}
