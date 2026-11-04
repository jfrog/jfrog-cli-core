package transferfiles

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockTransferSource struct {
	metadata       *SourceFileMetadata
	metadataErr    error
	reader         io.ReadCloser
	readerFactory  func() io.ReadCloser
	readerErr      error
	getReaderCalls int
}

func (m *mockTransferSource) GetFileMetadata(_ context.Context, _ api.FileRepresentation) (*SourceFileMetadata, error) {
	if m.metadataErr != nil {
		return nil, m.metadataErr
	}
	return m.metadata, nil
}

func (m *mockTransferSource) GetFileReader(_ context.Context, _ api.FileRepresentation) (io.ReadCloser, error) {
	m.getReaderCalls++
	if m.readerErr != nil {
		return nil, m.readerErr
	}
	if m.readerFactory != nil {
		return m.readerFactory(), nil
	}
	return m.reader, nil
}

type mockTransferTarget struct {
	checksumOutcome ChecksumDeployOutcome
	checksumErr     error
	checksumCalls   int
	putErr          error
	putFailAfter    int64
	putCalls        int
	putContent      []byte
	propsSkipped    bool
	propsErr        error
	propsCalls      int
	propsOptions    []TargetDeployOptions
	onApplyProps    func()
	statsErr        error
	statsCalls      int
	statsOptions    []TargetDeployOptions
	folderErr       error
	folderCalls     int
	releaseCalls    int
}

func (m *mockTransferTarget) TryChecksumDeploy(_ context.Context, _ *SourceFileMetadata, _ TargetDeployOptions) (ChecksumDeployOutcome, error) {
	m.checksumCalls++
	if m.checksumErr != nil {
		return ChecksumDeployMiss, m.checksumErr
	}
	return m.checksumOutcome, nil
}

func (m *mockTransferTarget) Put(_ context.Context, _ *SourceFileMetadata, reader io.Reader, _ TargetDeployOptions) error {
	m.putCalls++
	if m.putErr != nil {
		if m.putFailAfter > 0 {
			_, _ = io.CopyN(io.Discard, reader, m.putFailAfter)
		}
		return m.putErr
	}
	content, err := io.ReadAll(reader)
	if err == nil {
		m.putContent = content
	}
	return err
}

func (m *mockTransferTarget) CreateFolder(_ context.Context, _ *SourceFileMetadata, _ TargetDeployOptions) error {
	m.folderCalls++
	return m.folderErr
}

func (m *mockTransferTarget) ApplyProperties(_ context.Context, _ *SourceFileMetadata, options TargetDeployOptions) (bool, error) {
	m.propsCalls++
	m.propsOptions = append(m.propsOptions, options)
	if m.onApplyProps != nil {
		m.onApplyProps()
	}
	return m.propsSkipped, m.propsErr
}

func (m *mockTransferTarget) ApplyStatistics(_ context.Context, _ *SourceFileMetadata, options TargetDeployOptions) error {
	m.statsCalls++
	m.statsOptions = append(m.statsOptions, options)
	return m.statsErr
}

func (m *mockTransferTarget) ReleaseEligibleProperties(_ *SourceFileMetadata, _ TargetDeployOptions) {
	m.releaseCalls++
}

func (m *mockTransferTarget) HasEligibleProperties(metadata *SourceFileMetadata, options TargetDeployOptions) bool {
	eligible, _ := filterEligibleProperties(metadata.Properties, options)
	return eligible.KeysLen() > 0
}

func testFileCandidate() api.FileRepresentation {
	return api.FileRepresentation{
		Repo: testSourceRepo,
		Path: testSourcePath,
		Name: testSourceName,
		Size: 11,
	}
}

func testFileMetadata() *SourceFileMetadata {
	return &SourceFileMetadata{
		Repo:         testSourceRepo,
		Path:         testSourcePath,
		Name:         testSourceName,
		Size:         11,
		Sha1:         "sha1-value",
		Sha256:       "sha256-value",
		Md5:          "md5-value",
		Created:      "2020-01-01T00:00:00.000Z",
		CreatedBy:    "admin",
		LastModified: "2020-01-02T00:00:00.000Z",
		ModifiedBy:   "deployer",
		Properties: map[string][]string{
			"build.name": {"app"},
		},
	}
}

func TestFileTransfer_checksumHit_skipsGet(t *testing.T) {
	source := &mockTransferSource{metadata: testFileMetadata()}
	target := &mockTransferTarget{checksumOutcome: ChecksumDeployHit}

	ft := NewFileTransfer(source, target, FileTransferOptions{TargetDeployOptions: defaultTargetDeployOptions()})
	result := ft.TransferFile(context.Background(), testFileCandidate())

	require.NoError(t, result.Err)
	assert.Equal(t, api.Success, result.Status)
	assert.True(t, result.ChecksumDeployed)
	assert.Zero(t, result.BytesTransferred)
	assert.Equal(t, int64(11), result.FileSize)
	assert.GreaterOrEqual(t, result.DurationMillis, int64(0))
	assert.Equal(t, 1, target.checksumCalls)
	assert.Zero(t, source.getReaderCalls)
	assert.Equal(t, 1, target.propsCalls)
	assert.Equal(t, 1, target.statsCalls)

	status := result.ToFileUploadStatus()
	assert.Equal(t, int64(11), result.FileSize)
	assert.Equal(t, int64(11), status.SizeBytes)
}

func TestFileTransfer_checksumMiss_streamsThenPropertiesAndStats(t *testing.T) {
	payload := "hello world"
	source := &mockTransferSource{
		metadata: testFileMetadata(),
		reader:   io.NopCloser(strings.NewReader(payload)),
	}
	target := &mockTransferTarget{checksumOutcome: ChecksumDeployMiss}

	ft := NewFileTransfer(source, target, FileTransferOptions{TargetDeployOptions: defaultTargetDeployOptions()})
	result := ft.TransferFile(context.Background(), testFileCandidate())

	require.NoError(t, result.Err)
	assert.Equal(t, api.Success, result.Status)
	assert.False(t, result.ChecksumDeployed)
	assert.Equal(t, int64(len(payload)), result.BytesTransferred)
	assert.Equal(t, 1, target.checksumCalls)
	assert.Equal(t, 1, source.getReaderCalls)
	assert.Equal(t, 1, target.putCalls)
	assert.Equal(t, 1, target.propsCalls)
	assert.Equal(t, 1, target.statsCalls)
}

func TestFileTransfer_checksumBeforeGet(t *testing.T) {
	order := make([]string, 0, 2)
	source := &orderTrackingSource{
		mockTransferSource: mockTransferSource{
			metadata: testFileMetadata(),
			reader:   io.NopCloser(strings.NewReader("hello world")),
		},
		onGetReader: func() { order = append(order, "get") },
	}
	target := &orderTrackingTarget{
		mockTransferTarget: mockTransferTarget{checksumOutcome: ChecksumDeployMiss},
		onChecksum:         func() { order = append(order, "checksum") },
	}

	ft := NewFileTransfer(source, target, FileTransferOptions{TargetDeployOptions: defaultTargetDeployOptions()})
	_ = ft.TransferFile(context.Background(), testFileCandidate())
	assert.Equal(t, []string{"checksum", "get"}, order)
}

func TestFileTransfer_source404_isSkippedWithZeroSize(t *testing.T) {
	source := &mockTransferSource{metadataErr: ErrSourceItemGone}
	target := &mockTransferTarget{}

	ft := NewFileTransfer(source, target, FileTransferOptions{TargetDeployOptions: defaultTargetDeployOptions()})
	result := ft.TransferFile(context.Background(), testFileCandidate())

	require.NoError(t, result.Err)
	assert.Equal(t, api.SkippedSourceItemGone, result.Status)
	assert.True(t, result.SourceItemGone)
	assert.Zero(t, result.FileSize)
	assert.Zero(t, result.BytesTransferred)
	assert.Zero(t, target.checksumCalls)
	assert.Zero(t, source.getReaderCalls)
}

// TestFileTransfer_sourceItemGone_isCountedForSummary verifies that every gone-skip is counted so
// the end-of-run log line can report how many items were skipped as deleted.
func TestFileTransfer_sourceItemGone_isCountedForSummary(t *testing.T) {
	takeSourceItemGoneSkipCount() // reset
	ft := NewFileTransfer(&mockTransferSource{metadataErr: ErrSourceItemGone}, &mockTransferTarget{}, FileTransferOptions{TargetDeployOptions: defaultTargetDeployOptions()})
	_ = ft.TransferFile(context.Background(), testFileCandidate())

	ft = NewFileTransfer(&mockTransferSource{metadata: testFileMetadata(), readerErr: ErrSourceItemGone}, &mockTransferTarget{checksumOutcome: ChecksumDeployMiss}, FileTransferOptions{TargetDeployOptions: defaultTargetDeployOptions()})
	_ = ft.TransferFile(context.Background(), testFileCandidate())

	assert.Equal(t, int64(2), takeSourceItemGoneSkipCount())
	assert.Zero(t, takeSourceItemGoneSkipCount(), "taking the count resets it")
}

func TestTransferResult_ToFileUploadStatus_sourceItemGoneHasReason(t *testing.T) {
	result := TransferResult{Candidate: testFileCandidate(), Status: api.SkippedSourceItemGone, SourceItemGone: true}
	status := result.ToFileUploadStatus()
	assert.Equal(t, api.SkippedSourceItemGone, status.Status)
	assert.Equal(t, ErrSourceItemGone.Error(), status.Reason)
}

func TestFileTransfer_contentGet404_isSkippedWithZeroSize(t *testing.T) {
	source := &mockTransferSource{
		metadata:  testFileMetadata(),
		readerErr: ErrSourceItemGone,
	}
	target := &mockTransferTarget{checksumOutcome: ChecksumDeployMiss}

	ft := NewFileTransfer(source, target, FileTransferOptions{TargetDeployOptions: defaultTargetDeployOptions()})
	result := ft.TransferFile(context.Background(), testFileCandidate())

	require.NoError(t, result.Err)
	assert.Equal(t, api.SkippedSourceItemGone, result.Status)
	assert.True(t, result.SourceItemGone)
	assert.Zero(t, result.FileSize)
	assert.Zero(t, result.BytesTransferred)
	assert.Equal(t, 1, source.getReaderCalls)
	assert.Zero(t, target.putCalls)
}

func TestFileTransfer_putFailure_releasesEligibleProperties(t *testing.T) {
	putErr := errors.New("target put failed")
	source := &mockTransferSource{
		metadata: testFileMetadata(),
		reader:   io.NopCloser(strings.NewReader("hello world")),
	}
	target := &mockTransferTarget{checksumOutcome: ChecksumDeployMiss, putErr: putErr, putFailAfter: 3}

	ft := NewFileTransfer(source, target, FileTransferOptions{TargetDeployOptions: defaultTargetDeployOptions()})
	result := ft.TransferFile(context.Background(), testFileCandidate())

	require.ErrorIs(t, result.Err, putErr)
	assert.Equal(t, api.Fail, result.Status)
	assert.Zero(t, target.propsCalls)
	assert.Equal(t, 1, target.releaseCalls)
}

func TestFileTransfer_checksumDeployError_releasesEligibleProperties(t *testing.T) {
	checksumErr := errors.New("checksum deploy failed")
	source := &mockTransferSource{metadata: testFileMetadata()}
	target := &mockTransferTarget{checksumErr: checksumErr}

	ft := NewFileTransfer(source, target, FileTransferOptions{TargetDeployOptions: defaultTargetDeployOptions()})
	result := ft.TransferFile(context.Background(), testFileCandidate())

	require.ErrorIs(t, result.Err, checksumErr)
	assert.Equal(t, api.Fail, result.Status)
	assert.Zero(t, source.getReaderCalls)
	assert.Zero(t, target.propsCalls)
	assert.Equal(t, 1, target.releaseCalls)
}

func TestFileTransfer_createFolderError_releasesEligibleProperties(t *testing.T) {
	folderErr := errors.New("create folder failed")
	source := &mockTransferSource{metadata: testFolderMetadata()}
	target := &mockTransferTarget{folderErr: folderErr}

	ft := NewFileTransfer(source, target, FileTransferOptions{TargetDeployOptions: defaultTargetDeployOptions()})
	result := ft.TransferFile(context.Background(), testFolderCandidate(false))

	require.ErrorIs(t, result.Err, folderErr)
	assert.Equal(t, api.Fail, result.Status)
	assert.Zero(t, target.propsCalls)
	assert.Equal(t, 1, target.releaseCalls)
}

func TestFileTransfer_contentGet404_releasesTargetEligibleCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errors":[{"message":"not found"}]}`))
	}))
	defer server.Close()

	target, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	metadata := testFileMetadata()
	source := &mockTransferSource{metadata: metadata, readerErr: ErrSourceItemGone}
	options := defaultTargetDeployOptions()
	ft := NewFileTransfer(source, target, FileTransferOptions{TargetDeployOptions: options})
	result := ft.TransferFile(context.Background(), testFileCandidate())

	require.NoError(t, result.Err)
	assert.Equal(t, api.SkippedSourceItemGone, result.Status)
	assert.False(t, eligibleCacheHas(target, metadata, options),
		"TransferFile must release eligibleCache when content GET 404 skips ApplyProperties")
}

func TestFileTransfer_putFailure_isRetryable(t *testing.T) {
	putErr := errors.New("target put failed")
	source := &mockTransferSource{
		metadata: testFileMetadata(),
		reader:   io.NopCloser(strings.NewReader("hello world")),
	}
	target := &mockTransferTarget{checksumOutcome: ChecksumDeployMiss, putErr: putErr, putFailAfter: 3}

	ft := NewFileTransfer(source, target, FileTransferOptions{TargetDeployOptions: defaultTargetDeployOptions()})
	result := ft.TransferFile(context.Background(), testFileCandidate())

	require.ErrorIs(t, result.Err, putErr)
	assert.Equal(t, api.Fail, result.Status)
	assert.Zero(t, result.BytesTransferred)
	assert.Empty(t, target.putContent)
}

func TestFileTransfer_putSuccess_propertiesFailure_keepsTransferredBytes(t *testing.T) {
	payload := "hello world"
	propsErr := errors.New("apply properties failed")
	source := &mockTransferSource{
		metadata: testFileMetadata(),
		reader:   io.NopCloser(strings.NewReader(payload)),
	}
	target := &mockTransferTarget{checksumOutcome: ChecksumDeployMiss, propsErr: propsErr}

	ft := NewFileTransfer(source, target, FileTransferOptions{TargetDeployOptions: defaultTargetDeployOptions()})
	result := ft.TransferFile(context.Background(), testFileCandidate())

	require.ErrorIs(t, result.Err, propsErr)
	assert.Equal(t, api.Fail, result.Status)
	assert.Equal(t, payload, string(target.putContent))
	assert.Equal(t, 1, target.putCalls)
	assert.Equal(t, 1, target.propsCalls)
	assert.Zero(t, target.statsCalls)
	assert.Equal(t, int64(len(payload)), result.BytesTransferred)
}

func TestFileTransfer_putSuccess_statisticsFailure_keepsTransferredBytes(t *testing.T) {
	payload := "hello world"
	statsErr := errors.New("apply statistics failed")
	source := &mockTransferSource{
		metadata: testFileMetadata(),
		reader:   io.NopCloser(strings.NewReader(payload)),
	}
	target := &mockTransferTarget{checksumOutcome: ChecksumDeployMiss, statsErr: statsErr}

	ft := NewFileTransfer(source, target, FileTransferOptions{TargetDeployOptions: defaultTargetDeployOptions()})
	result := ft.TransferFile(context.Background(), testFileCandidate())

	require.ErrorIs(t, result.Err, statsErr)
	assert.Equal(t, api.Fail, result.Status)
	assert.Equal(t, payload, string(target.putContent))
	assert.Equal(t, 1, target.putCalls)
	assert.Equal(t, 1, target.propsCalls)
	assert.Equal(t, 1, target.statsCalls)
	assert.Equal(t, int64(len(payload)), result.BytesTransferred)
}

func TestFileTransfer_retryOpensFreshGet(t *testing.T) {
	payload := "hello world"
	var sources []*closeTrackingReadCloser
	source := &mockTransferSource{
		metadata: testFileMetadata(),
		readerFactory: func() io.ReadCloser {
			reader := &closeTrackingReadCloser{ReadCloser: io.NopCloser(strings.NewReader(payload))}
			sources = append(sources, reader)
			return reader
		},
	}
	putErr := errors.New("mid-stream PUT failure")
	target := &mockTransferTarget{checksumOutcome: ChecksumDeployMiss, putErr: putErr, putFailAfter: 3}

	ft := NewFileTransfer(source, target, FileTransferOptions{TargetDeployOptions: defaultTargetDeployOptions()})

	result1 := ft.TransferFile(context.Background(), testFileCandidate())
	require.ErrorIs(t, result1.Err, putErr)
	assert.Zero(t, result1.BytesTransferred)
	assert.Empty(t, target.putContent)
	require.Len(t, sources, 1)
	assert.True(t, sources[0].closed.Load())

	target.putErr = nil
	result2 := ft.TransferFile(context.Background(), testFileCandidate())
	require.NoError(t, result2.Err)
	assert.Equal(t, 2, source.getReaderCalls)
	require.Len(t, sources, 2)
	assert.NotSame(t, sources[0], sources[1])
	assert.Equal(t, payload, string(target.putContent))
}

func TestFileTransfer_configureOptions_isConcurrentSafe(t *testing.T) {
	ft := NewFileTransfer(nil, nil, FileTransferOptions{})
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func(bufferSize int) {
			defer wg.Done()
			ft.ConfigureOptions(FileTransferOptions{StreamBufferSize: bufferSize})
		}(i + 1)
		go func() {
			defer wg.Done()
			_ = ft.getOptions()
		}()
	}

	wg.Wait()
}

func TestFileTransfer_statisticsUsesTransferOptionsSnapshot(t *testing.T) {
	initialOptions := FileTransferOptions{
		TargetDeployOptions: TargetDeployOptions{PackageType: "generic"},
	}
	updatedOptions := FileTransferOptions{
		TargetDeployOptions: TargetDeployOptions{PackageType: "buildinfo", BuildInfoRepo: true},
	}
	source := &mockTransferSource{metadata: testFileMetadata()}
	target := &mockTransferTarget{checksumOutcome: ChecksumDeployHit}
	ft := NewFileTransfer(source, target, initialOptions)
	target.onApplyProps = func() {
		ft.ConfigureOptions(updatedOptions)
	}

	result := ft.TransferFile(context.Background(), testFileCandidate())

	require.NoError(t, result.Err)
	require.Equal(t, []TargetDeployOptions{initialOptions.TargetDeployOptions}, target.propsOptions)
	require.Equal(t, []TargetDeployOptions{initialOptions.TargetDeployOptions}, target.statsOptions)
}

func TestFileTransfer_canceledContextAfterGetReader_closesReader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reader := &closeTrackingReadCloser{ReadCloser: io.NopCloser(strings.NewReader("hello world"))}
	source := &mockTransferSource{
		metadata: testFileMetadata(),
		readerFactory: func() io.ReadCloser {
			cancel()
			return reader
		},
	}
	target := &mockTransferTarget{checksumOutcome: ChecksumDeployMiss}

	ft := NewFileTransfer(source, target, FileTransferOptions{TargetDeployOptions: defaultTargetDeployOptions()})
	result := ft.TransferFile(ctx, testFileCandidate())

	require.ErrorIs(t, result.Err, context.Canceled)
	assert.Equal(t, api.Fail, result.Status)
	assert.Zero(t, target.putCalls)
	assert.Equal(t, 1, target.releaseCalls)
	assert.True(t, reader.closed.Load())
	assert.Equal(t, int32(1), reader.closeCalls.Load())
}

func TestFileTransfer_inFlightCancellationReturnsContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	source := &cancelAwareTransferSource{started: make(chan struct{})}
	ft := NewFileTransfer(source, &mockTransferTarget{}, FileTransferOptions{})
	result := make(chan TransferResult, 1)

	go func() {
		result <- ft.TransferFile(ctx, testFileCandidate())
	}()
	<-source.started
	cancel()

	select {
	case transferResult := <-result:
		require.ErrorIs(t, transferResult.Err, context.Canceled)
		assert.Equal(t, api.Fail, transferResult.Status)
	case <-time.After(time.Second):
		t.Fatal("in-flight transfer did not observe context cancellation")
	}
}

func TestFileTransfer_toFileUploadStatus_success(t *testing.T) {
	result := TransferResult{
		Candidate:        testFileCandidate(),
		Status:           api.Success,
		ChecksumDeployed: true,
		BytesTransferred: 0,
		FileSize:         testFileCandidate().Size,
	}
	status := result.ToFileUploadStatus()
	assert.Equal(t, api.Success, status.Status)
	assert.True(t, status.ChecksumDeployed)
	assert.Equal(t, testFileCandidate().Repo, status.Repo)
	assert.Equal(t, testFileCandidate().Size, status.SizeBytes)
}

func TestFileTransfer_checksumHit_toFileUploadStatus_usesLogicalFileSize(t *testing.T) {
	candidate := api.FileRepresentation{
		Repo: testSourceRepo,
		Path: testSourcePath,
		Name: testSourceName,
		Size: 42,
	}
	source := &mockTransferSource{metadata: &SourceFileMetadata{
		Repo: testSourceRepo,
		Path: testSourcePath,
		Name: testSourceName,
		Size: 42,
	}}
	target := &mockTransferTarget{checksumOutcome: ChecksumDeployHit}

	ft := NewFileTransfer(source, target, FileTransferOptions{TargetDeployOptions: defaultTargetDeployOptions()})
	result := ft.TransferFile(context.Background(), candidate)

	require.NoError(t, result.Err)
	assert.Zero(t, result.BytesTransferred)
	assert.Equal(t, int64(42), result.FileSize)
	assert.Equal(t, int64(42), result.ToFileUploadStatus().SizeBytes)
}

func testFolderCandidate(nonEmpty bool) api.FileRepresentation {
	return api.FileRepresentation{
		Repo:        testSourceRepo,
		Path:        "empty-dir",
		NonEmptyDir: nonEmpty,
	}
}

func testFolderMetadata() *SourceFileMetadata {
	return &SourceFileMetadata{
		Repo: testSourceRepo,
		Path: "empty-dir",
		Name: "",
		Size: 0,
	}
}

func TestFileTransfer_folderNonEmptyDirNoProperties_skipsCreateFolder(t *testing.T) {
	source := &mockTransferSource{metadata: testFolderMetadata()}
	target := &mockTransferTarget{}

	ft := NewFileTransfer(source, target, FileTransferOptions{TargetDeployOptions: defaultTargetDeployOptions()})
	result := ft.TransferFile(context.Background(), testFolderCandidate(true))

	require.NoError(t, result.Err)
	assert.Equal(t, api.SkippedNonEmptyDir, result.Status)
	assert.Zero(t, target.folderCalls)
	assert.Zero(t, target.propsCalls)
	assert.Zero(t, target.statsCalls)
	assert.GreaterOrEqual(t, result.DurationMillis, int64(0))
}

func TestFileTransfer_folderNonEmptyDirWithProperties_createsFolder(t *testing.T) {
	metadata := testFolderMetadata()
	metadata.Properties = map[string][]string{
		"build.name": {"app"},
	}
	source := &mockTransferSource{metadata: metadata}
	target := &mockTransferTarget{}

	ft := NewFileTransfer(source, target, FileTransferOptions{TargetDeployOptions: defaultTargetDeployOptions()})
	result := ft.TransferFile(context.Background(), testFolderCandidate(true))

	require.NoError(t, result.Err)
	assert.Equal(t, api.Success, result.Status)
	assert.Equal(t, 1, target.folderCalls)
	assert.Equal(t, 1, target.propsCalls)
	assert.Equal(t, 1, target.statsCalls)
}

func TestFileTransfer_emptyFolder_createsFolder(t *testing.T) {
	source := &mockTransferSource{metadata: testFolderMetadata()}
	target := &mockTransferTarget{}

	ft := NewFileTransfer(source, target, FileTransferOptions{TargetDeployOptions: defaultTargetDeployOptions()})
	result := ft.TransferFile(context.Background(), testFolderCandidate(false))

	require.NoError(t, result.Err)
	assert.Equal(t, api.Success, result.Status)
	assert.Equal(t, 1, target.folderCalls)
	assert.Equal(t, 1, target.propsCalls)
	assert.Equal(t, 1, target.statsCalls)
}

type orderTrackingSource struct {
	mockTransferSource
	onGetReader func()
}

func (s *orderTrackingSource) GetFileReader(ctx context.Context, file api.FileRepresentation) (io.ReadCloser, error) {
	if s.onGetReader != nil {
		s.onGetReader()
	}
	return s.mockTransferSource.GetFileReader(ctx, file)
}

type orderTrackingTarget struct {
	mockTransferTarget
	onChecksum func()
}

type cancelAwareTransferSource struct {
	started chan struct{}
}

func (s *cancelAwareTransferSource) GetFileMetadata(ctx context.Context, _ api.FileRepresentation) (*SourceFileMetadata, error) {
	close(s.started)
	<-ctx.Done()
	return nil, nil
}

func (s *cancelAwareTransferSource) GetFileReader(_ context.Context, _ api.FileRepresentation) (io.ReadCloser, error) {
	return nil, errors.New("unexpected GetFileReader call")
}

func (t *orderTrackingTarget) TryChecksumDeploy(ctx context.Context, metadata *SourceFileMetadata, options TargetDeployOptions) (ChecksumDeployOutcome, error) {
	if t.onChecksum != nil {
		t.onChecksum()
	}
	return t.mockTransferTarget.TryChecksumDeploy(ctx, metadata, options)
}
