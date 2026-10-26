package transferfiles

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamGetToPut_success_exactSize(t *testing.T) {
	payload := strings.Repeat("a", 100)
	source := &closeTrackingReadCloser{ReadCloser: io.NopCloser(strings.NewReader(payload))}
	var received []byte

	n, err := StreamGetToPut(context.Background(), int64(len(payload)), source, func(_ context.Context, reader io.Reader) error {
		var readErr error
		received, readErr = readAllWithBuffer(reader, defaultStreamBufferSize)
		return readErr
	}, defaultStreamBufferSize)
	require.NoError(t, err)
	assert.Equal(t, int64(len(payload)), n)
	assert.Equal(t, payload, string(received))
	assert.Equal(t, int32(1), source.closeCalls.Load())
}

func TestStreamGetToPut_shortBody_fails(t *testing.T) {
	source := io.NopCloser(strings.NewReader("short"))
	_, err := StreamGetToPut(context.Background(), 100, source, func(_ context.Context, reader io.Reader) error {
		_, copyErr := io.Copy(io.Discard, reader)
		return copyErr
	}, defaultStreamBufferSize)
	require.Error(t, err)
	assert.True(t, IsStreamSizeMismatch(err))
}

func TestStreamGetToPut_longBody_fails(t *testing.T) {
	source := io.NopCloser(strings.NewReader(strings.Repeat("x", 101)))
	_, err := StreamGetToPut(context.Background(), 100, source, func(_ context.Context, reader io.Reader) error {
		_, copyErr := io.Copy(io.Discard, reader)
		return copyErr
	}, defaultStreamBufferSize)
	require.Error(t, err)
	assert.True(t, IsStreamSizeMismatch(err))
}

func TestStreamGetToPut_putError_closesSource(t *testing.T) {
	sourceClosed := false
	source := &closeTrackingReadCloser{
		ReadCloser: io.NopCloser(strings.NewReader("payload")),
		onClose: func() {
			sourceClosed = true
		},
	}

	putErr := errors.New("put failed")
	_, err := StreamGetToPut(context.Background(), 7, source, func(_ context.Context, _ io.Reader) error {
		return putErr
	}, defaultStreamBufferSize)
	require.ErrorIs(t, err, putErr)
	assert.True(t, sourceClosed)
	assert.Equal(t, int32(1), source.closeCalls.Load())
}

func TestStreamGetToPut_putError_unblocksSourceRead(t *testing.T) {
	source := newCloseUnblocksReadCloser()
	putErr := errors.New("put failed")
	result := make(chan error, 1)

	go func() {
		_, err := StreamGetToPut(context.Background(), 1, source, func(_ context.Context, _ io.Reader) error {
			<-source.readStarted
			return putErr
		}, defaultStreamBufferSize)
		result <- err
	}()

	select {
	case err := <-result:
		require.ErrorIs(t, err, putErr)
		assert.True(t, source.closed.Load())
		assert.Equal(t, int32(1), source.closeCalls.Load())
	case <-time.After(time.Second):
		t.Fatal("stream transfer remained blocked after PUT failure")
	}
}

func TestStreamGetToPut_cancel_closesBothLegs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	blockRead := make(chan struct{})

	source := &blockingReadCloser{
		data:  strings.Repeat("z", 1024),
		block: blockRead,
	}
	var putReader io.Reader
	putStarted := make(chan struct{})

	go func() {
		<-putStarted
		cancel()
		close(blockRead)
	}()

	_, err := StreamGetToPut(ctx, 1024, source, func(_ context.Context, reader io.Reader) error {
		putReader = reader
		close(putStarted)
		_, readErr := reader.Read(make([]byte, 1))
		return readErr
	}, defaultStreamBufferSize)
	require.ErrorIs(t, err, context.Canceled)
	assert.True(t, source.closed)
	require.NotNil(t, putReader)
}

// TestStreamGetToPut_cancel_withCtxBlindSourceAndRealTargetPut reproduces the deadlock the
// fake-put cancel tests above cannot detect: a source reader that is genuinely ctx-blind (a real
// network response body, not a type that selects on ctx internally) feeding a real
// TargetClient.Put over the wire. Before the fix, ctx cancellation only closed source/pw after
// put returned, but put could never return because its writeLoop was blocked reading pr, which
// only unblocks once the copy goroutine writes to pw, which never happens while source.Read is
// itself stuck. This must complete well within the 2s bound instead of hanging forever.
func TestStreamGetToPut_cancel_withCtxBlindSourceAndRealTargetPut(t *testing.T) {
	// source.Read blocks on a channel and is ctx-blind: nothing but an explicit Close unblocks
	// it, exactly like a stalled real network reader.
	source := newCloseUnblocksReadCloser()

	targetServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer targetServer.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(targetServer.URL), nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	putStarted := make(chan struct{})
	go func() {
		<-putStarted
		<-source.readStarted
		cancel()
	}()

	metadata := testTargetMetadata()
	metadata.Size = 1024

	start := time.Now()
	_, err = StreamGetToPut(ctx, metadata.Size, source, func(putCtx context.Context, reader io.Reader) error {
		close(putStarted)
		return client.Put(putCtx, metadata, reader, defaultTargetDeployOptions())
	}, defaultStreamBufferSize)
	elapsed := time.Since(start)
	require.Error(t, err)
	assert.True(t, source.closed.Load())
	assert.Less(t, elapsed, 2*time.Second,
		"StreamGetToPut must not hang when ctx cancels while a ctx-blind source Read and the real target PUT's write are both mid-flight")
}

func TestStreamGetToPut_usesBoundedBuffer_notReadAll(t *testing.T) {
	payload := strings.Repeat("b", defaultStreamBufferSize*3+17)
	source := io.NopCloser(strings.NewReader(payload))
	var maxRead int

	n, err := StreamGetToPut(context.Background(), int64(len(payload)), source, func(_ context.Context, reader io.Reader) error {
		buf := make([]byte, defaultStreamBufferSize)
		for {
			readN, readErr := reader.Read(buf)
			if readN > maxRead {
				maxRead = readN
			}
			if readErr == io.EOF {
				return nil
			}
			if readErr != nil {
				return readErr
			}
		}
	}, defaultStreamBufferSize)
	require.NoError(t, err)
	assert.Equal(t, int64(len(payload)), n)
	assert.LessOrEqual(t, maxRead, defaultStreamBufferSize)
}

func TestStreamGetToPut_putPanic_closesSourceAndUnblocksCopy(t *testing.T) {
	source := newCloseUnblocksReadCloser()
	result := make(chan error, 1)

	go func() {
		_, err := StreamGetToPut(context.Background(), 1, source, func(_ context.Context, _ io.Reader) error {
			<-source.readStarted
			panic("put exploded")
		}, defaultStreamBufferSize)
		result <- err
	}()

	select {
	case err := <-result:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "panic in put")
		assert.True(t, source.closed.Load())
		assert.Equal(t, int32(1), source.closeCalls.Load())
	case <-time.After(time.Second):
		t.Fatal("stream transfer remained blocked after put panic")
	}
}

func setStreamIdleTimeout(t *testing.T, timeout time.Duration) {
	orig := streamIdleTimeout
	streamIdleTimeout = timeout
	t.Cleanup(func() { streamIdleTimeout = orig })
}

func TestStreamGetToPut_sourceStallsMidBody_failsWithIdleError(t *testing.T) {
	setStreamIdleTimeout(t, 100*time.Millisecond)
	source := newCloseUnblocksReadCloser()
	result := make(chan error, 1)

	go func() {
		_, err := StreamGetToPut(context.Background(), 1, source, func(_ context.Context, reader io.Reader) error {
			_, copyErr := io.Copy(io.Discard, reader)
			return copyErr
		}, defaultStreamBufferSize)
		result <- err
	}()

	select {
	case err := <-result:
		require.ErrorIs(t, err, ErrStreamIdle)
		assert.True(t, source.closed.Load())
	case <-time.After(2 * time.Second):
		t.Fatal("stream transfer remained blocked on a stalled source")
	}
}

func TestStreamGetToPut_targetStopsReading_failsWithIdleError(t *testing.T) {
	setStreamIdleTimeout(t, 100*time.Millisecond)
	payload := strings.Repeat("t", 4*defaultStreamBufferSize)
	source := &closeTrackingReadCloser{ReadCloser: io.NopCloser(strings.NewReader(payload))}
	result := make(chan error, 1)

	go func() {
		_, err := StreamGetToPut(context.Background(), int64(len(payload)), source, func(putCtx context.Context, reader io.Reader) error {
			if _, readErr := reader.Read(make([]byte, 1)); readErr != nil {
				return readErr
			}
			// A target that stops reading the body blocks the HTTP client until its context ends.
			<-putCtx.Done()
			return putCtx.Err()
		}, defaultStreamBufferSize)
		result <- err
	}()

	select {
	case err := <-result:
		require.ErrorIs(t, err, ErrStreamIdle)
		assert.True(t, source.closed.Load())
	case <-time.After(2 * time.Second):
		t.Fatal("stream transfer remained blocked on a target that stopped reading")
	}
}

func TestStreamGetToPut_slowButProgressingStream_succeeds(t *testing.T) {
	setStreamIdleTimeout(t, 150*time.Millisecond)
	payload := "slow"
	source := io.NopCloser(&trickleReader{data: payload, delay: 50 * time.Millisecond})

	n, err := StreamGetToPut(context.Background(), int64(len(payload)), source, func(_ context.Context, reader io.Reader) error {
		_, copyErr := io.Copy(io.Discard, reader)
		return copyErr
	}, defaultStreamBufferSize)
	require.NoError(t, err)
	assert.Equal(t, int64(len(payload)), n)
}

func TestStreamGetToPut_slowTargetResponseAfterBody_isNotIdle(t *testing.T) {
	setStreamIdleTimeout(t, 50*time.Millisecond)
	payload := "payload"
	source := io.NopCloser(strings.NewReader(payload))

	n, err := StreamGetToPut(context.Background(), int64(len(payload)), source, func(putCtx context.Context, reader io.Reader) error {
		if _, copyErr := io.Copy(io.Discard, reader); copyErr != nil {
			return copyErr
		}
		// The whole body was sent; the target may take long to answer (e.g. checksumming a large
		// file). Waiting for the response is bounded by ResponseHeaderTimeout, not the idle timeout.
		time.Sleep(200 * time.Millisecond)
		return putCtx.Err()
	}, defaultStreamBufferSize)
	require.NoError(t, err)
	assert.Equal(t, int64(len(payload)), n)
}

// trickleReader returns one byte per Read, after waiting delay.
type trickleReader struct {
	data  string
	delay time.Duration
}

func (r *trickleReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	time.Sleep(r.delay)
	n := copy(p[:1], r.data)
	r.data = r.data[n:]
	return n, nil
}

func TestStreamGetToPut_largeGeneratedStream_remainsBufferBounded(t *testing.T) {
	const streamSize = int64(64 * 1024 * 1024)
	source := &generatedReadCloser{remaining: streamSize}

	n, err := StreamGetToPut(context.Background(), streamSize, source, func(_ context.Context, reader io.Reader) error {
		_, copyErr := io.CopyBuffer(io.Discard, reader, make([]byte, defaultStreamBufferSize))
		return copyErr
	}, defaultStreamBufferSize)

	require.NoError(t, err)
	assert.Equal(t, streamSize, n)
	assert.LessOrEqual(t, source.maxReadRequest, defaultStreamBufferSize)
	assert.True(t, source.closed.Load())
}

func TestStreamGetToPut_retryIssuesFreshReader(t *testing.T) {
	var getCount atomic.Int32
	payload := strings.Repeat("retry-me", defaultStreamBufferSize)
	sources := make([]*closeTrackingReadCloser, 0, 2)

	get := func() io.ReadCloser {
		getCount.Add(1)
		source := &closeTrackingReadCloser{ReadCloser: io.NopCloser(strings.NewReader(payload))}
		sources = append(sources, source)
		return source
	}

	putErr := errors.New("mid-stream PUT failure")
	var committed string
	firstPut := func(_ context.Context, reader io.Reader) error {
		partial := make([]byte, 17)
		_, err := io.ReadFull(reader, partial)
		require.NoError(t, err)
		return putErr
	}

	_, err := StreamGetToPut(context.Background(), int64(len(payload)), get(), firstPut, defaultStreamBufferSize)
	require.ErrorIs(t, err, putErr)
	require.Len(t, sources, 1)
	assert.True(t, sources[0].closed.Load())
	assert.Empty(t, committed)

	_, err = StreamGetToPut(context.Background(), int64(len(payload)), get(), func(_ context.Context, reader io.Reader) error {
		content, readErr := io.ReadAll(reader)
		if readErr == nil {
			committed = string(content)
		}
		return readErr
	}, defaultStreamBufferSize)
	require.NoError(t, err)
	assert.Equal(t, int32(2), getCount.Load())
	assert.Equal(t, payload, committed)
}

type generatedReadCloser struct {
	remaining      int64
	maxReadRequest int
	closed         atomic.Bool
}

func (r *generatedReadCloser) Read(p []byte) (int, error) {
	if len(p) > r.maxReadRequest {
		r.maxReadRequest = len(p)
	}
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := len(p)
	if int64(n) > r.remaining {
		n = int(r.remaining)
	}
	for i := 0; i < n; i++ {
		p[i] = 'x'
	}
	r.remaining -= int64(n)
	return n, nil
}

func (r *generatedReadCloser) Close() error {
	r.closed.Store(true)
	return nil
}

type closeTrackingReadCloser struct {
	io.ReadCloser
	onClose    func()
	closed     atomic.Bool
	closeCalls atomic.Int32
}

func (c *closeTrackingReadCloser) Close() error {
	c.closeCalls.Add(1)
	c.closed.Store(true)
	if c.onClose != nil {
		c.onClose()
	}
	if c.ReadCloser != nil {
		return c.ReadCloser.Close()
	}
	return nil
}

type closeUnblocksReadCloser struct {
	readStarted chan struct{}
	unblockRead chan struct{}
	closeOnce   sync.Once
	closed      atomic.Bool
	closeCalls  atomic.Int32
}

func newCloseUnblocksReadCloser() *closeUnblocksReadCloser {
	return &closeUnblocksReadCloser{
		readStarted: make(chan struct{}),
		unblockRead: make(chan struct{}),
	}
}

func (c *closeUnblocksReadCloser) Read(_ []byte) (int, error) {
	select {
	case <-c.readStarted:
	default:
		close(c.readStarted)
	}
	<-c.unblockRead
	return 0, io.ErrClosedPipe
}

func (c *closeUnblocksReadCloser) Close() error {
	c.closeCalls.Add(1)
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		close(c.unblockRead)
	})
	return nil
}

type blockingReadCloser struct {
	data   string
	offset int
	block  chan struct{}
	closed bool
}

func (b *blockingReadCloser) Read(p []byte) (int, error) {
	<-b.block
	if b.offset >= len(b.data) {
		return 0, io.EOF
	}
	n := copy(p, b.data[b.offset:])
	b.offset += n
	return n, nil
}

func (b *blockingReadCloser) Close() error {
	b.closed = true
	return nil
}

func readAllWithBuffer(r io.Reader, bufSize int) ([]byte, error) {
	buf := make([]byte, bufSize)
	var out []byte
	for {
		n, err := r.Read(buf)
		if n > 0 {
			out = append(out, buf[:n]...)
		}
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return out, err
		}
	}
}
