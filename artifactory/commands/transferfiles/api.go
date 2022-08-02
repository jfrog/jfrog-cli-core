package transferfiles

import (
	"encoding/json"
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
	UuidTokens []string `json:"uuid_tokens,omitempty"`
}

type UploadChunksStatusResponse struct {
	NodeIdResponse
	ChunksStatus []ChunkStatus `json:"chunks_status,omitempty"`
}

type ChunkStatus struct {
	UuidTokenResponse
	Status ProcessStatusType          `json:"status,omitempty"`
	Files  []FileUploadStatusResponse `json:"files,omitempty"`
}

type FileUploadStatusResponse struct {
	FileRepresentation
	Status     ChunkFileStatusType `json:"status,omitempty"`
	StatusCode int                 `json:"status_code,omitempty"`
	Reason     string              `json:"reason,omitempty"`
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

// Fill tokens batch till full. Return if no new tokens are available.
func (ucs *UploadChunksStatusBody) fillTokensBatch(uploadTokensChan chan string) {
	for len(ucs.UuidTokens) < getThreads() {
		select {
		case token := <-uploadTokensChan:
			ucs.UuidTokens = append(ucs.UuidTokens, token)
		default:
			// No new tokens are waiting.
			return
		}
	}
}

func (uc *UploadChunk) appendUploadCandidate(file FileRepresentation) {
	uc.UploadCandidates = append(uc.UploadCandidates, file)
}
