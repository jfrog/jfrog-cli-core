package transferfiles

import (
	"bytes"
	"context"
	"crypto/md5"  // #nosec G501 -- MD5 is an Artifactory checksum type, not used for security
	"crypto/sha1" // #nosec G505 -- SHA-1 is an Artifactory checksum type, not used for security
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type harnessBlob struct {
	sha1          string
	sha256        string
	md5           string
	size          int64
	body          []byte
	generatedByte byte
	generated     bool
	properties    map[string][]string
}

type transferHTTPHarness struct {
	mu sync.Mutex

	sourceStore  map[string]*harnessBlob
	targetStore  map[string]*harnessBlob
	targetBySha1 map[string]*harnessBlob
	targetStaged map[string][]byte

	sourceDownloadGETs    int
	sourceReaderClosed    atomic.Bool
	sourceReaderClosedCh  chan struct{}
	sourceReaderCloseOnce sync.Once
	sourceMaxReadLen      atomic.Int64
	checksumPUTs          int
	checksumPutBodyBytes  int64
	fullPUTs              int
	failFirstFullPutAfter int
	fullPutAttempts       int

	afterFullPutBytes  func(n int)
	blockFullPutAfter  int
	fullPutBlocked     chan struct{}
	releaseFullPut     chan struct{}
	blockFullPutOnce   sync.Once
	fullPutDone        chan struct{}
	fullPutDoneOnce    sync.Once
	discardFullPutBody bool

	sourceServer *httptest.Server
	targetServer *httptest.Server
}

func newTransferHTTPHarness(t *testing.T) *transferHTTPHarness {
	t.Helper()
	h := &transferHTTPHarness{
		sourceStore:          make(map[string]*harnessBlob),
		targetStore:          make(map[string]*harnessBlob),
		targetBySha1:         make(map[string]*harnessBlob),
		targetStaged:         make(map[string][]byte),
		sourceReaderClosedCh: make(chan struct{}),
	}
	h.sourceServer = httptest.NewServer(http.HandlerFunc(h.serveSource))
	h.targetServer = httptest.NewServer(http.HandlerFunc(h.serveTarget))
	t.Cleanup(func() {
		h.sourceServer.Close()
		h.targetServer.Close()
	})
	return h
}

func (h *transferHTTPHarness) seedGeneratedSource(relativePath string, size int64, b byte) *harnessBlob {
	sha1Sum, sha256Sum, md5Sum := checksumsForRepeatingBytes(size, b)
	blob := &harnessBlob{
		sha1:          sha1Sum,
		sha256:        sha256Sum,
		md5:           md5Sum,
		size:          size,
		generatedByte: b,
		generated:     true,
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sourceStore[relativePath] = blob
	return blob
}

func (h *transferHTTPHarness) seedSource(relativePath string, body []byte, properties map[string][]string) *harnessBlob {
	blob := newHarnessBlob(body, properties)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sourceStore[relativePath] = blob
	return blob
}

func (h *transferHTTPHarness) seedTarget(relativePath string, body []byte, properties map[string][]string) *harnessBlob {
	blob := newHarnessBlob(body, properties)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.targetStore[relativePath] = blob
	h.targetBySha1[blob.sha1] = cloneHarnessBlob(blob)
	return blob
}

func (h *transferHTTPHarness) seedTargetBySha1(body []byte) *harnessBlob {
	blob := newHarnessBlob(body, nil)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.targetBySha1[blob.sha1] = cloneHarnessBlob(blob)
	return blob
}

func (h *transferHTTPHarness) targetBlob(relativePath string) *harnessBlob {
	h.mu.Lock()
	defer h.mu.Unlock()
	return cloneHarnessBlob(h.targetStore[relativePath])
}

func (h *transferHTTPHarness) stagedTargetContent(relativePath string) []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]byte(nil), h.targetStaged[relativePath]...)
}

type closeTrackingTransferSource struct {
	fileTransferSource
	onClose    func()
	maxReadLen *atomic.Int64
}

func (s *closeTrackingTransferSource) GetFileReader(ctx context.Context, file api.FileRepresentation) (io.ReadCloser, error) {
	reader, err := s.fileTransferSource.GetFileReader(ctx, file)
	if err != nil {
		return nil, err
	}
	if s.maxReadLen != nil {
		reader = &readSizeTrackingReadCloser{ReadCloser: reader, maxReadLen: s.maxReadLen}
	}
	return &closeTrackingReadCloser{
		ReadCloser: reader,
		onClose:    s.onClose,
	}, nil
}

// readSizeTrackingReadCloser records the largest single Read() buffer requested by the caller.
// Unlike a heap-growth sample, this is a deterministic proof that the source is consumed in
// bounded chunks (<= defaultStreamBufferSize) rather than read whole into memory at once.
type readSizeTrackingReadCloser struct {
	io.ReadCloser
	maxReadLen *atomic.Int64
}

func (r *readSizeTrackingReadCloser) Read(p []byte) (int, error) {
	requested := int64(len(p))
	for {
		old := r.maxReadLen.Load()
		if requested <= old || r.maxReadLen.CompareAndSwap(old, requested) {
			break
		}
	}
	return r.ReadCloser.Read(p)
}

func (h *transferHTTPHarness) newFileTransfer(t *testing.T, options ...FileTransferOptions) *FileTransfer {
	t.Helper()
	opts := FileTransferOptions{TargetDeployOptions: defaultTargetDeployOptions()}
	if len(options) > 0 {
		opts = options[0]
	}
	sourceClient, err := NewSourceClient(context.Background(), newTestSourceServerDetails(h.sourceServer.URL))
	require.NoError(t, err)
	source := &closeTrackingTransferSource{
		fileTransferSource: sourceClient,
		onClose: func() {
			h.sourceReaderClosed.Store(true)
			h.sourceReaderCloseOnce.Do(func() { close(h.sourceReaderClosedCh) })
		},
		maxReadLen: &h.sourceMaxReadLen,
	}
	targetClient, err := NewTargetClient(context.Background(), newTestTargetServerDetails(h.targetServer.URL), nil)
	require.NoError(t, err)
	return NewFileTransfer(source, targetClient, opts)
}

func (h *transferHTTPHarness) serveSource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/storage/") {
		h.serveSourceStorage(w, r)
		return
	}
	relativePath := strings.TrimPrefix(r.URL.Path, "/")
	h.mu.Lock()
	blob, ok := h.sourceStore[relativePath]
	if ok {
		h.sourceDownloadGETs++
	}
	h.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.WriteHeader(http.StatusOK)
	if blob.generated {
		_, _ = io.Copy(w, io.LimitReader(repeatingByteReader{b: blob.generatedByte}, blob.size))
		return
	}
	_, _ = w.Write(blob.body)
}

func (h *transferHTTPHarness) serveSourceStorage(w http.ResponseWriter, r *http.Request) {
	relativePath := strings.TrimPrefix(r.URL.Path, "/api/storage/")
	h.mu.Lock()
	blob, ok := h.sourceStore[relativePath]
	h.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	switch r.URL.RawQuery {
	case "properties":
		props := blob.properties
		if props == nil {
			props = map[string][]string{}
		}
		w.WriteHeader(http.StatusOK)
		writeTestJSON(w, map[string]any{"properties": props})
	case "stats":
		w.WriteHeader(http.StatusOK)
		writeTestJSON(w, map[string]any{
			"downloadCount":    0,
			"lastDownloaded":   0,
			"lastDownloadedBy": "",
		})
	case "":
		w.WriteHeader(http.StatusOK)
		writeTestJSON(w, map[string]any{
			"repo":         testSourceRepo,
			"path":         testSourcePath + "/" + testSourceName,
			"created":      "2020-01-01T00:00:00.000Z",
			"createdBy":    "admin",
			"lastModified": "2020-01-02T00:00:00.000Z",
			"modifiedBy":   "deployer",
			"size":         strconv.FormatInt(blob.size, 10),
			"checksums": map[string]string{
				"sha1":   blob.sha1,
				"sha256": blob.sha256,
				"md5":    blob.md5,
			},
		})
	default:
		http.NotFound(w, r)
	}
}

func (h *transferHTTPHarness) serveTarget(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/system/ping" {
		_, _ = w.Write([]byte("OK"))
		return
	}
	if r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/metadata/") {
		h.serveTargetPropertiesPatch(w, r)
		return
	}
	if r.Method != http.MethodPut {
		http.NotFound(w, r)
		return
	}
	itemPath := strings.TrimPrefix(stripMatrixParams(r.URL.Path), "/")
	if strings.HasSuffix(itemPath, itemStatisticsSuffix) {
		w.WriteHeader(http.StatusCreated)
		return
	}
	matrixProps := parseMatrixProperties(r.URL.Path)
	if r.Header.Get("X-Checksum-Deploy") == "true" {
		h.serveChecksumDeploy(w, r, itemPath, matrixProps)
		return
	}
	h.serveFullPut(w, r, itemPath, matrixProps)
}

func (h *transferHTTPHarness) serveChecksumDeploy(w http.ResponseWriter, r *http.Request, itemPath string, matrixProps map[string][]string) {
	body, _ := io.ReadAll(r.Body)
	sha1Header := r.Header.Get("X-Checksum-Sha1")
	h.mu.Lock()
	h.checksumPUTs++
	h.checksumPutBodyBytes += int64(len(body))
	existing, ok := h.targetBySha1[sha1Header]
	if ok {
		stored := cloneHarnessBlob(existing)
		stored.properties = mergeProperties(stored.properties, matrixProps)
		h.targetStore[itemPath] = stored
	}
	h.mu.Unlock()
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *transferHTTPHarness) serveFullPut(w http.ResponseWriter, r *http.Request, itemPath string, matrixProps map[string][]string) {
	if h.fullPutDone != nil {
		defer h.fullPutDoneOnce.Do(func() { close(h.fullPutDone) })
	}
	h.mu.Lock()
	h.fullPutAttempts++
	attempt := h.fullPutAttempts
	failAfter := h.failFirstFullPutAfter
	afterBytes := h.afterFullPutBytes
	discardBody := h.discardFullPutBody
	h.mu.Unlock()

	var buf bytes.Buffer
	sha1Hash := sha1.New() // #nosec G401 -- Artifactory checksum type, not used for security
	sha256Hash := sha256.New()
	md5Hash := md5.New() // #nosec G401 -- Artifactory checksum type, not used for security
	var received int64
	defer func() {
		h.mu.Lock()
		delete(h.targetStaged, itemPath)
		h.mu.Unlock()
	}()
	chunk := make([]byte, 32*1024)
	for {
		n, err := r.Body.Read(chunk)
		if n > 0 {
			received += int64(n)
			if discardBody {
				_, _ = sha1Hash.Write(chunk[:n])
				_, _ = sha256Hash.Write(chunk[:n])
				_, _ = md5Hash.Write(chunk[:n])
			} else {
				_, _ = buf.Write(chunk[:n])
				h.mu.Lock()
				h.targetStaged[itemPath] = append([]byte(nil), buf.Bytes()...)
				h.mu.Unlock()
			}
			if afterBytes != nil {
				afterBytes(int(received))
			}
			if h.blockFullPutAfter > 0 && received >= int64(h.blockFullPutAfter) {
				h.blockFullPutOnce.Do(func() { close(h.fullPutBlocked) })
				<-h.releaseFullPut
			}
			if failAfter > 0 && attempt == 1 && received >= int64(failAfter) {
				if hj, ok := w.(http.Hijacker); ok {
					conn, _, hijackErr := hj.Hijack()
					if hijackErr == nil {
						_ = conn.Close()
					}
				}
				return
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return
		}
	}

	if contentLength := r.Header.Get("Content-Length"); contentLength != "" {
		want, convErr := strconv.Atoi(contentLength)
		if convErr == nil && int64(want) != received {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	var blob *harnessBlob
	if discardBody {
		blob = &harnessBlob{
			sha1:       hex.EncodeToString(sha1Hash.Sum(nil)),
			sha256:     hex.EncodeToString(sha256Hash.Sum(nil)),
			md5:        hex.EncodeToString(md5Hash.Sum(nil)),
			size:       received,
			properties: cloneProperties(matrixProps),
		}
	} else {
		blob = newHarnessBlob(buf.Bytes(), matrixProps)
	}

	h.mu.Lock()
	h.fullPUTs++
	h.targetStore[itemPath] = blob
	h.targetBySha1[blob.sha1] = cloneHarnessBlob(blob)
	h.mu.Unlock()
	w.WriteHeader(http.StatusCreated)
}

func (h *transferHTTPHarness) serveTargetPropertiesPatch(w http.ResponseWriter, r *http.Request) {
	itemPath := strings.TrimPrefix(r.URL.Path, "/api/metadata/")
	var payload updateItemPropertiesBody
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	existing := h.targetStore[itemPath]
	if existing == nil {
		existing = &harnessBlob{properties: map[string][]string{}}
		h.targetStore[itemPath] = existing
	}
	existing.properties = mergeProperties(existing.properties, payload.Props)
	w.WriteHeader(http.StatusNoContent)
}

func newHarnessBlob(body []byte, properties map[string][]string) *harnessBlob {
	sha1Sum := sha1.Sum(body) // #nosec G401 -- Artifactory checksum type, not used for security
	sha256Sum := sha256.Sum256(body)
	md5Sum := md5.Sum(body) // #nosec G401 -- Artifactory checksum type, not used for security
	return &harnessBlob{
		sha1:       hex.EncodeToString(sha1Sum[:]),
		sha256:     hex.EncodeToString(sha256Sum[:]),
		md5:        hex.EncodeToString(md5Sum[:]),
		size:       int64(len(body)),
		body:       append([]byte(nil), body...),
		properties: cloneProperties(properties),
	}
}

func cloneHarnessBlob(blob *harnessBlob) *harnessBlob {
	if blob == nil {
		return nil
	}
	return &harnessBlob{
		sha1:          blob.sha1,
		sha256:        blob.sha256,
		md5:           blob.md5,
		size:          blob.size,
		body:          append([]byte(nil), blob.body...),
		generatedByte: blob.generatedByte,
		generated:     blob.generated,
		properties:    cloneProperties(blob.properties),
	}
}

type repeatingByteReader struct {
	b byte
}

func (r repeatingByteReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = r.b
	}
	return len(p), nil
}

func checksumsForRepeatingBytes(size int64, b byte) (sha1Sum, sha256Sum, md5Sum string) {
	sha1Hash := sha1.New() // #nosec G401 -- Artifactory checksum type, not used for security
	sha256Hash := sha256.New()
	md5Hash := md5.New() // #nosec G401 -- Artifactory checksum type, not used for security
	_, _ = io.Copy(io.MultiWriter(sha1Hash, sha256Hash, md5Hash), io.LimitReader(repeatingByteReader{b: b}, size))
	return hex.EncodeToString(sha1Hash.Sum(nil)), hex.EncodeToString(sha256Hash.Sum(nil)), hex.EncodeToString(md5Hash.Sum(nil))
}

func cloneProperties(properties map[string][]string) map[string][]string {
	if properties == nil {
		return nil
	}
	cloned := maps.Clone(properties)
	for key, values := range cloned {
		cloned[key] = append([]string(nil), values...)
	}
	return cloned
}

func mergeProperties(existing, extra map[string][]string) map[string][]string {
	merged := cloneProperties(existing)
	if merged == nil {
		merged = map[string][]string{}
	}
	for key, values := range extra {
		merged[key] = append([]string(nil), values...)
	}
	return merged
}

func stripMatrixParams(path string) string {
	if i := strings.Index(path, ";"); i >= 0 {
		return path[:i]
	}
	return path
}

func parseMatrixProperties(path string) map[string][]string {
	props := map[string][]string{}
	semicolon := strings.Index(path, ";")
	if semicolon < 0 {
		return props
	}
	for _, segment := range strings.Split(path[semicolon+1:], ";") {
		if segment == "" {
			continue
		}
		key, value, ok := strings.Cut(segment, "=")
		if !ok {
			continue
		}
		decodedKey, err := url.QueryUnescape(key)
		if err != nil {
			decodedKey = key
		}
		decodedValue, err := url.QueryUnescape(value)
		if err != nil {
			decodedValue = value
		}
		props[decodedKey] = append(props[decodedKey], decodedValue)
	}
	return props
}

func TestFileTransferHTTP_largeStream_matchesSourceSha1AndSize(t *testing.T) {
	h := newTransferHTTPHarness(t)
	payload := bytes.Repeat([]byte("a"), defaultStreamBufferSize*4)
	sourceBlob := h.seedSource(testSourceRelativePath(), payload, nil)

	ft := h.newFileTransfer(t)
	candidate := testFileCandidate()
	candidate.Size = sourceBlob.size
	result := ft.TransferFile(context.Background(), candidate)

	require.NoError(t, result.Err)
	assert.Equal(t, api.Success, result.Status)
	stored := h.targetBlob(testSourceRelativePath())
	require.NotNil(t, stored)
	assert.Equal(t, sourceBlob.sha1, stored.sha1)
	assert.Equal(t, sourceBlob.size, stored.size)
	assert.Equal(t, payload, stored.body)
	h.mu.Lock()
	downloadGETs := h.sourceDownloadGETs
	fullPUTs := h.fullPUTs
	h.mu.Unlock()
	assert.GreaterOrEqual(t, downloadGETs, 1)
	assert.GreaterOrEqual(t, fullPUTs, 1)
}

func TestFileTransferHTTP_largeStream_peakHeapStaysBelowFullBuffering(t *testing.T) {
	h := newTransferHTTPHarness(t)
	h.discardFullPutBody = true
	const payloadSize int64 = 8 << 20
	sourceBlob := h.seedGeneratedSource(testSourceRelativePath(), payloadSize, 'm')

	var peak atomic.Uint64
	stop := make(chan struct{})
	var sampler sync.WaitGroup
	sampler.Add(1)
	go func() {
		defer sampler.Done()
		var ms runtime.MemStats
		for {
			runtime.ReadMemStats(&ms)
			for {
				old := peak.Load()
				if ms.HeapAlloc <= old || peak.CompareAndSwap(old, ms.HeapAlloc) {
					break
				}
			}
			select {
			case <-stop:
				return
			case <-time.After(2 * time.Millisecond):
			}
		}
	}()

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	peak.Store(before.HeapAlloc)

	ft := h.newFileTransfer(t)
	candidate := testFileCandidate()
	candidate.Size = payloadSize
	result := ft.TransferFile(context.Background(), candidate)
	close(stop)
	sampler.Wait()

	require.NoError(t, result.Err)
	assert.Equal(t, api.Success, result.Status)
	stored := h.targetBlob(testSourceRelativePath())
	require.NotNil(t, stored)
	assert.Equal(t, sourceBlob.sha1, stored.sha1)
	assert.Equal(t, payloadSize, stored.size)
	assert.Empty(t, stored.body)

	growth := int64(peak.Load()) - int64(before.HeapAlloc) // #nosec G115 -- test heap sizes fit in int64
	if growth < 0 {
		growth = 0
	}
	assert.Less(t, growth, payloadSize, "peak heap grew by %d bytes; full-buffering the payload would retain ~%d", growth, payloadSize)

	// Deterministic complement to the heap-growth sample above: heap sampling can be noisy
	// (GC timing, unrelated allocations), so also prove directly that the source is never asked
	// to fill a buffer larger than the stream's fixed chunk size, i.e. it is never read whole.
	assert.LessOrEqual(t, h.sourceMaxReadLen.Load(), int64(defaultStreamBufferSize),
		"source Read() was asked to fill a buffer of %d bytes; expected reads bounded by the %d-byte stream buffer",
		h.sourceMaxReadLen.Load(), defaultStreamBufferSize)
}

func TestFileTransferHTTP_midPutFail_retryHasNoPartialAndFreshGet(t *testing.T) {
	h := newTransferHTTPHarness(t)
	payload := bytes.Repeat([]byte("b"), defaultStreamBufferSize*3)
	sourceBlob := h.seedSource(testSourceRelativePath(), payload, nil)
	previous := h.seedTarget(testSourceRelativePath(), []byte("previous-complete-blob"), nil)
	h.failFirstFullPutAfter = defaultStreamBufferSize

	ft := h.newFileTransfer(t)
	candidate := testFileCandidate()
	candidate.Size = sourceBlob.size

	failed := ft.TransferFile(context.Background(), candidate)
	require.Error(t, failed.Err)
	assert.Equal(t, api.Fail, failed.Status)
	afterFail := h.targetBlob(testSourceRelativePath())
	require.NotNil(t, afterFail)
	assert.Equal(t, previous.body, afterFail.body)
	assert.Equal(t, previous.sha1, afterFail.sha1)
	assert.True(t, h.sourceReaderClosed.Load(), "failed attempt must close its source response body")
	assert.Empty(t, h.stagedTargetContent(testSourceRelativePath()), "failed attempt must remove staged target bytes")

	h.sourceReaderClosed.Store(false)
	retried := ft.TransferFile(context.Background(), candidate)
	require.NoError(t, retried.Err)
	assert.Equal(t, api.Success, retried.Status)
	stored := h.targetBlob(testSourceRelativePath())
	require.NotNil(t, stored)
	assert.Equal(t, sourceBlob.sha1, stored.sha1)
	assert.Equal(t, sourceBlob.size, stored.size)
	assert.Equal(t, payload, stored.body)
	h.mu.Lock()
	downloadGETs := h.sourceDownloadGETs
	h.mu.Unlock()
	assert.GreaterOrEqual(t, downloadGETs, 2)
}

func TestFileTransferHTTP_checksumHit_streamsZeroBytes(t *testing.T) {
	h := newTransferHTTPHarness(t)
	payload := []byte("hello world")
	sourceBlob := h.seedSource(testSourceRelativePath(), payload, map[string][]string{
		"build.name": {"app"},
	})
	h.seedTargetBySha1(payload)

	ft := h.newFileTransfer(t)
	result := ft.TransferFile(context.Background(), testFileCandidate())

	require.NoError(t, result.Err)
	assert.Equal(t, api.Success, result.Status)
	assert.True(t, result.ChecksumDeployed)
	assert.Zero(t, result.BytesTransferred)
	h.mu.Lock()
	downloadGETs := h.sourceDownloadGETs
	checksumPUTs := h.checksumPUTs
	checksumBodyBytes := h.checksumPutBodyBytes
	fullPUTs := h.fullPUTs
	h.mu.Unlock()
	assert.Zero(t, downloadGETs)
	assert.GreaterOrEqual(t, checksumPUTs, 1)
	assert.Zero(t, checksumBodyBytes)
	assert.Zero(t, fullPUTs)
	stored := h.targetBlob(testSourceRelativePath())
	require.NotNil(t, stored)
	assert.Equal(t, sourceBlob.sha1, stored.sha1)
}

func TestFileTransferHTTP_interrupt_leavesNoStagedContent(t *testing.T) {
	h := newTransferHTTPHarness(t)
	payload := bytes.Repeat([]byte("c"), defaultStreamBufferSize*256)
	sourceBlob := h.seedSource(testSourceRelativePath(), payload, nil)
	h.blockFullPutAfter = defaultStreamBufferSize
	h.fullPutBlocked = make(chan struct{})
	h.releaseFullPut = make(chan struct{})
	h.fullPutDone = make(chan struct{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ft := h.newFileTransfer(t)
	candidate := testFileCandidate()
	candidate.Size = sourceBlob.size
	resultCh := make(chan TransferResult, 1)
	go func() {
		resultCh <- ft.TransferFile(ctx, candidate)
	}()

	var midStreamStaged []byte
	select {
	case <-h.fullPutBlocked:
		// Captured before cancel(), while the target handler is parked mid-write: this proves
		// a genuine partial write reached the target (not zero bytes, not the full payload),
		// i.e. the interruption is exercised mid-stream rather than before or after any data flowed.
		midStreamStaged = h.stagedTargetContent(testSourceRelativePath())
		cancel()
	case <-time.After(time.Second):
		close(h.releaseFullPut)
		t.Fatal("target PUT did not reach the synchronized interruption point")
	}
	assert.NotEmpty(t, midStreamStaged, "target handler should have staged partial bytes before the interruption point")
	assert.Less(t, int64(len(midStreamStaged)), sourceBlob.size, "target handler should not have received the full payload before the interruption point")

	select {
	case <-h.sourceReaderClosedCh:
	case <-time.After(time.Second):
		close(h.releaseFullPut)
		t.Fatal("cancellation did not close the source response")
	}
	close(h.releaseFullPut)

	var result TransferResult
	select {
	case result = <-resultCh:
	case <-time.After(time.Second):
		t.Fatal("interrupted transfer did not return")
	}
	select {
	case <-h.fullPutDone:
	case <-time.After(time.Second):
		t.Fatal("target handler did not discard staged content")
	}

	require.Error(t, result.Err)
	assert.Equal(t, api.Fail, result.Status)
	assert.True(t, h.sourceReaderClosed.Load())
	assert.Nil(t, h.targetBlob(testSourceRelativePath()))
	// stagedTargetContent is not re-checked here: serveFullPut unconditionally deletes it via
	// defer on handler return, success or failure alike, so an empty read at this point would be
	// guaranteed by the harness regardless of production behavior. The meaningful assertion - that
	// real partial content existed before the interruption - is made above, while still in-flight.
}

func TestFileTransferHTTP_propertiesLandOnTarget(t *testing.T) {
	h := newTransferHTTPHarness(t)
	payload := []byte("hello world")
	h.seedSource(testSourceRelativePath(), payload, map[string][]string{
		"build.name": {"app"},
		"env":        {"prod"},
		"npm.name":   {"generated-should-drop"},
	})

	ft := h.newFileTransfer(t, FileTransferOptions{TargetDeployOptions: TargetDeployOptions{
		MinChecksumDeploySize: 10,
		PackageType:           "npm",
	}})
	result := ft.TransferFile(context.Background(), testFileCandidate())

	require.NoError(t, result.Err)
	assert.Equal(t, api.Success, result.Status)
	stored := h.targetBlob(testSourceRelativePath())
	require.NotNil(t, stored)
	assert.Equal(t, map[string][]string{
		"build.name": {"app"},
		"env":        {"prod"},
	}, stored.properties)
	assert.NotContains(t, stored.properties, "npm.name")
}

// writeTestJSON encodes v to w, reporting an encoding failure to the client instead of ignoring it.
func writeTestJSON(w http.ResponseWriter, v map[string]any) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
