package transferfiles

import (
	"encoding/json"
	"fmt"

	"github.com/jfrog/jfrog-client-go/utils/log"
)

type ProcessStatusType string

const (
	Done       ProcessStatusType = "DONE"
	InProgress ProcessStatusType = "IN_PROGRESS"
)

type ChunkFileStatusType string

const (
	Success             ChunkFileStatusType = "SUCCESS"
	Fail                ChunkFileStatusType = "FAIL"
	SkippedLargeProps   ChunkFileStatusType = "SKIPPED_LARGE_PROPS"
	SkippedMetadataFile ChunkFileStatusType = "SKIPPED_METADATA_FILE"
)

type TargetAuth struct {
	TargetArtifactoryUrl string `json:"target_artifactory_url,omitempty"`
	TargetUsername       string `json:"target_username,omitempty"`
	TargetPassword       string `json:"target_password,omitempty"`
	TargetToken          string `json:"target_token,omitempty"`
	TargetProxyKey       string `json:"target_proxy_key,omitempty"`
}

type HandlePropertiesDiff struct {
	TargetAuth
	RepoKey           string `json:"repo_key,omitempty"`
	StartMilliseconds string `json:"start_milliseconds,omitempty"`
	EndMilliseconds   string `json:"end_milliseconds,omitempty"`
}

type HandlePropertiesDiffResponse struct {
	NodeIdResponse
	PropertiesDelivered json.Number               `json:"properties_delivered,omitempty"`
	PropertiesTotal     json.Number               `json:"properties_total,omitempty"`
	Status              ProcessStatusType         `json:"status,omitempty"`
	Errors              []PropertiesHandlingError `json:"errors,omitempty"`
}

type PropertiesHandlingError struct {
	FileRepresentation
	StatusCode string `json:"status_code,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

type UploadChunk struct {
	TargetAuth
	CheckExistenceInFilestore bool                 `json:"check_existence_in_filestore,omitempty"`
	UploadCandidates          []FileRepresentation `json:"upload_candidates,omitempty"`
}

type FileRepresentation struct {
	Repo string `json:"repo,omitempty"`
	Path string `json:"path,omitempty"`
	Name string `json:"name,omitempty"`
}

type UploadChunkResponse struct {
	NodeIdResponse
	UuidTokenResponse
}

type UploadChunksStatusBody struct {
	AwaitingStatusChunks []chunkId `json:"awaiting_status_chunks,omitempty"`
	ChunksToDelete       []chunkId `json:"chunks_to_delete,omitempty"`
}

type UploadChunksStatusResponse struct {
	NodeIdResponse
	ChunksStatus  []ChunkStatus `json:"chunks_status,omitempty"`
	DeletedChunks []string      `json:"deleted_chunks,omitempty"`
}

type ChunkStatus struct {
	UuidTokenResponse
	Status         ProcessStatusType          `json:"status,omitempty"`
	Files          []FileUploadStatusResponse `json:"files,omitempty"`
	DurationMillis int64                      `json:"duration_millis,omitempty"`
}

type FileUploadStatusResponse struct {
	FileRepresentation
	SizeBytes        int64               `json:"size_bytes,omitempty"`
	ChecksumDeployed bool                `json:"checksum_deployed,omitempty"`
	Status           ChunkFileStatusType `json:"status,omitempty"`
	StatusCode       int                 `json:"status_code,omitempty"`
	Reason           string              `json:"reason,omitempty"`
}

type FilesErrors struct {
	Errors []ExtendedFileUploadStatusResponse `json:"errors,omitempty"`
}

type NodeIdResponse struct {
	NodeId string `json:"node_id,omitempty"`
}

type UuidTokenResponse struct {
	UuidToken string `json:"uuid_token,omitempty"`
}

// Fill chunk data batch till full. Return if no new chunk data is available.
func fillChunkDataBatch(chunksResilientManager *ChunksLifeCycleManager, uploadChunkChan chan UploadedChunkData) {
	for chunksResilientManager.totalChunks < GetThreads() {
		select {
		case data := <-uploadChunkChan:
			currentNodeId := nodeId(data.NodeId)
			currentChunkId := chunkId(data.ChunkUuid)
			if _, exist := chunksResilientManager.nodeToChunksMap[currentNodeId]; !exist {
				chunksResilientManager.nodeToChunksMap[currentNodeId] = map[chunkId][]FileRepresentation{}
			}
			chunksResilientManager.nodeToChunksMap[currentNodeId][currentChunkId] =
				append(chunksResilientManager.nodeToChunksMap[currentNodeId][currentChunkId], data.ChunkFiles...)
			chunksResilientManager.totalChunks++
		default:
			// No new tokens are waiting.
			return
		}
	}
}

// Append upload candidate to the list of upload candidates. Skip empty directories in build-info repositories.
// file          - The upload candidate
// buildInfoRepo - True if this is a build-info repository
func (uc *UploadChunk) appendUploadCandidateIfNeeded(file FileRepresentation, buildInfoRepo bool) {
	if buildInfoRepo && file.Name == "" {
		log.Debug(fmt.Sprintf("Skipping unneeded empty dir '%s' in the build-info repository '%s'", file.Path, file.Repo))
		return
	}
	uc.UploadCandidates = append(uc.UploadCandidates, file)
}
