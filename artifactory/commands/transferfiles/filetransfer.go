package transferfiles

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type fileTransferSource interface {
	GetFileMetadata(ctx context.Context, file api.FileRepresentation) (*SourceFileMetadata, error)
	GetFileReader(ctx context.Context, file api.FileRepresentation) (io.ReadCloser, error)
}

type fileTransferTarget interface {
	TryChecksumDeploy(ctx context.Context, metadata *SourceFileMetadata, options TargetDeployOptions) (ChecksumDeployOutcome, error)
	Put(ctx context.Context, metadata *SourceFileMetadata, reader io.Reader, options TargetDeployOptions) error
	CreateFolder(ctx context.Context, metadata *SourceFileMetadata, options TargetDeployOptions) error
	ApplyProperties(ctx context.Context, metadata *SourceFileMetadata, options TargetDeployOptions) (skippedLargeProps bool, err error)
	ApplyStatistics(ctx context.Context, metadata *SourceFileMetadata, options TargetDeployOptions) error
	ReleaseEligibleProperties(metadata *SourceFileMetadata, options TargetDeployOptions)
	HasEligibleProperties(metadata *SourceFileMetadata, options TargetDeployOptions) bool
}

type FileTransferOptions struct {
	TargetDeployOptions TargetDeployOptions
	StreamBufferSize    int
}

type TransferResult struct {
	Candidate         api.FileRepresentation
	Status            api.ChunkFileStatusType
	Err               error
	ChecksumDeployed  bool
	BytesTransferred  int64
	FileSize          int64
	DurationMillis    int64
	SkippedLargeProps bool
	SourceItemGone    bool
}

func (r TransferResult) ToFileUploadStatus() api.FileUploadStatusResponse {
	status := api.FileUploadStatusResponse{
		FileRepresentation: r.Candidate,
		SizeBytes:          r.FileSize,
		ChecksumDeployed:   r.ChecksumDeployed,
		Status:             r.Status,
	}
	if r.Err != nil {
		status.Reason = r.Err.Error()
	} else if r.SourceItemGone {
		status.Reason = ErrSourceItemGone.Error()
	}
	return status
}

type FileTransfer struct {
	source      fileTransferSource
	target      fileTransferTarget
	options     FileTransferOptions
	optionsLock sync.RWMutex
}

func NewFileTransfer(source fileTransferSource, target fileTransferTarget, options FileTransferOptions) *FileTransfer {
	return &FileTransfer{
		source:  source,
		target:  target,
		options: options,
	}
}

// ConfigureOptions updates deploy options for subsequent transfers.
func (ft *FileTransfer) ConfigureOptions(options FileTransferOptions) {
	ft.optionsLock.Lock()
	defer ft.optionsLock.Unlock()
	ft.options = options
}

// sourceItemGoneSkipCount counts items skipped as "source item gone" during this run, so the
// end-of-run summary can tell the user how many were skipped.
var sourceItemGoneSkipCount atomic.Int64

// takeSourceItemGoneSkipCount returns the number of items skipped as "source item gone" since
// the last call, and resets the counter.
func takeSourceItemGoneSkipCount() int64 {
	return sourceItemGoneSkipCount.Swap(0)
}

// logSourceItemGone records every source-item-gone skip so it's auditable post-run instead
// of vanishing silently into a "success" summary.
func logSourceItemGone(candidate api.FileRepresentation, stage string) {
	sourceItemGoneSkipCount.Add(1)
	log.Warn("Source item no longer exists, skipping:", fileRelativePath(candidate)+". Detected while:", stage)
}

func (ft *FileTransfer) TransferFile(ctx context.Context, candidate api.FileRepresentation) TransferResult {
	startTime := time.Now()
	result := TransferResult{Candidate: candidate, Status: api.Success}
	options := ft.getOptions()
	if err := ctx.Err(); err != nil {
		return ft.finalizeResult(ft.failResult(result, err), startTime, nil, candidate)
	}

	metadata, err := ft.source.GetFileMetadata(ctx, candidate)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ft.finalizeResult(ft.failResult(result, errors.Join(err, ctxErr)), startTime, nil, candidate)
	}
	if err != nil {
		if IsSourceItemGone(err) {
			logSourceItemGone(candidate, "fetching metadata")
			result.SourceItemGone = true
			result.Status = api.SkippedSourceItemGone
			return ft.finalizeResult(result, startTime, nil, candidate)
		}
		return ft.finalizeResult(ft.failResult(result, err), startTime, nil, candidate)
	}
	defer ft.target.ReleaseEligibleProperties(metadata, options.TargetDeployOptions)

	if metadata.Name == "" {
		return ft.transferFolder(ctx, result, metadata, startTime, candidate, options)
	}

	outcome, err := ft.target.TryChecksumDeploy(ctx, metadata, options.TargetDeployOptions)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ft.finalizeResult(ft.failResult(result, errors.Join(err, ctxErr)), startTime, metadata, candidate)
	}
	if err != nil {
		return ft.finalizeResult(ft.failResult(result, err), startTime, metadata, candidate)
	}
	if outcome == ChecksumDeployHit {
		result.ChecksumDeployed = true
		return ft.finalizeResult(ft.applyPropertiesAndStats(ctx, result, metadata, options), startTime, metadata, candidate)
	}

	reader, err := ft.source.GetFileReader(ctx, candidate)
	if ctxErr := ctx.Err(); ctxErr != nil {
		closeReadCloser(reader)
		return ft.finalizeResult(ft.failResult(result, errors.Join(err, ctxErr)), startTime, metadata, candidate)
	}
	if err != nil {
		closeReadCloser(reader)
		if IsSourceItemGone(err) {
			logSourceItemGone(candidate, "reading file content")
			result.SourceItemGone = true
			result.Status = api.SkippedSourceItemGone
			return ft.finalizeResult(result, startTime, metadata, candidate)
		}
		return ft.finalizeResult(ft.failResult(result, err), startTime, metadata, candidate)
	}

	bufferSize := options.StreamBufferSize
	if bufferSize <= 0 {
		bufferSize = defaultStreamBufferSize
	}
	bytesTransferred, err := StreamGetToPut(ctx, metadata.Size, reader, func(streamCtx context.Context, streamReader io.Reader) error {
		return ft.target.Put(streamCtx, metadata, streamReader, options.TargetDeployOptions)
	}, bufferSize)
	if err != nil {
		return ft.finalizeResult(ft.failResult(result, err), startTime, metadata, candidate)
	}
	result.BytesTransferred = bytesTransferred

	return ft.finalizeResult(ft.applyPropertiesAndStats(ctx, result, metadata, options), startTime, metadata, candidate)
}

func (ft *FileTransfer) transferFolder(ctx context.Context, result TransferResult, metadata *SourceFileMetadata, startTime time.Time, candidate api.FileRepresentation, options FileTransferOptions) TransferResult {
	if candidate.NonEmptyDir && !ft.target.HasEligibleProperties(metadata, options.TargetDeployOptions) {
		result.Status = api.SkippedNonEmptyDir
		return ft.finalizeResult(result, startTime, metadata, candidate)
	}
	err := ft.target.CreateFolder(ctx, metadata, options.TargetDeployOptions)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ft.finalizeResult(ft.failResult(result, errors.Join(err, ctxErr)), startTime, metadata, candidate)
	}
	if err != nil {
		return ft.finalizeResult(ft.failResult(result, err), startTime, metadata, candidate)
	}
	return ft.finalizeResult(ft.applyPropertiesAndStats(ctx, result, metadata, options), startTime, metadata, candidate)
}

func (ft *FileTransfer) applyPropertiesAndStats(ctx context.Context, result TransferResult, metadata *SourceFileMetadata, options FileTransferOptions) TransferResult {
	skippedLargeProps, err := ft.target.ApplyProperties(ctx, metadata, options.TargetDeployOptions)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return failAfterContentTransfer(result, errors.Join(err, ctxErr))
	}
	if err != nil {
		return failAfterContentTransfer(result, err)
	}
	result.SkippedLargeProps = skippedLargeProps
	if skippedLargeProps {
		result.Status = api.SkippedLargeProps
	}
	err = ft.target.ApplyStatistics(ctx, metadata, options.TargetDeployOptions)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return failAfterContentTransfer(result, errors.Join(err, ctxErr))
	}
	if err != nil {
		return failAfterContentTransfer(result, err)
	}
	return result
}

func (ft *FileTransfer) getOptions() FileTransferOptions {
	ft.optionsLock.RLock()
	defer ft.optionsLock.RUnlock()
	return ft.options
}

func (ft *FileTransfer) failResult(result TransferResult, err error) TransferResult {
	result.Status = api.Fail
	result.Err = err
	result.BytesTransferred = 0
	return result
}

// failAfterContentTransfer marks the item failed without clearing BytesTransferred.
// Content may already have landed on the target (checksum-deploy hit or a completed stream)
// before properties/stats failed.
func failAfterContentTransfer(result TransferResult, err error) TransferResult {
	result.Status = api.Fail
	result.Err = err
	return result
}

func (ft *FileTransfer) finalizeResult(result TransferResult, startTime time.Time, metadata *SourceFileMetadata, candidate api.FileRepresentation) TransferResult {
	result.DurationMillis = time.Since(startTime).Milliseconds()
	if result.SourceItemGone {
		result.FileSize = 0
		return result
	}
	if metadata != nil {
		result.FileSize = metadata.Size
	} else {
		result.FileSize = candidate.Size
	}
	return result
}

var (
	_ fileTransferSource = (*SourceClient)(nil)
	_ fileTransferTarget = (*TargetClient)(nil)
)
