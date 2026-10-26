package transferfiles

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

const defaultStreamBufferSize = 32 * 1024

// streamIdleTimeout fails a stream when no bytes move from the source to the target for this long
// while the body is being copied. The streaming clients have no overall timeout (see
// streamResponseHeaderTimeout), so without it a source that stops sending, or a target that stops
// reading, mid-body while keeping its connection open would block the worker until the whole
// transfer is cancelled. It does not apply once the body is fully copied: the wait for the target's
// response is bounded by streamResponseHeaderTimeout.
//
// A var (not a const) so tests can shrink it.
var streamIdleTimeout = 5 * time.Minute

var ErrStreamSizeMismatch = errors.New("stream body size mismatch")

var ErrStreamIdle = errors.New("stream stalled")

func IsStreamSizeMismatch(err error) bool {
	return errors.Is(err, ErrStreamSizeMismatch)
}

type putFunc func(ctx context.Context, reader io.Reader) error

// StreamGetToPut pipes source GET bytes into target PUT using a bounded buffer.
// expectedSize must match the exact number of bytes read from source.
func StreamGetToPut(ctx context.Context, expectedSize int64, source io.ReadCloser, put putFunc, bufferSize int) (int64, error) {
	if bufferSize <= 0 {
		bufferSize = defaultStreamBufferSize
	}
	if source == nil {
		return 0, fmt.Errorf("source reader is nil")
	}

	// put runs under its own context so a stalled stream can abort a target PUT that is blocked
	// writing to a target which stopped reading.
	putCtx, cancelPut := context.WithCancelCause(ctx)
	defer cancelPut(nil)

	pr, pw := io.Pipe()
	progress := &progressWriter{w: pw}
	progress.touch()
	copyDone := make(chan streamCopyResult, 1)
	copyFinished := make(chan struct{})
	buf := make([]byte, bufferSize)

	go func() {
		var result streamCopyResult
		defer func() {
			close(copyFinished)
			copyDone <- result
		}()
		defer closePipeWriter(pw, &result.err)

		result.bytesCopied, result.err = copyWithExactSize(ctx, progress, source, expectedSize, buf)
	}()

	// put(ctx, pr) can block on pr.Read(), which only unblocks once the copy goroutine above
	// writes to (or closes) pw. If source.Read() is itself blocked and ctx-blind, that copy
	// goroutine never reaches pw, so a ctx cancellation would otherwise hang here forever:
	// nothing closes source or pw until after put returns, and put can't return until one of
	// them is closed. This watcher breaks that deadlock by closing both as soon as ctx is done,
	// without waiting for put to return first. It does the same, and also cancels put, when no
	// bytes have moved for streamIdleTimeout while the body is still being copied.
	idleTimeout := streamIdleTimeout
	watchDone := make(chan struct{})
	go func() {
		idleCheck := time.NewTicker(max(idleTimeout/4, time.Millisecond))
		defer idleCheck.Stop()
		idleTicks := idleCheck.C
		finished := copyFinished
		for {
			select {
			case <-ctx.Done():
				closeReadCloser(source)
				_ = pw.CloseWithError(ctx.Err())
				return
			case <-watchDone:
				return
			case <-finished:
				finished, idleTicks = nil, nil
			case <-idleTicks:
				if progress.idleFor() < idleTimeout {
					continue
				}
				select {
				case <-finished:
					finished, idleTicks = nil, nil
					continue
				default:
				}
				idleErr := fmt.Errorf("%w: no bytes were transferred for %s", ErrStreamIdle, idleTimeout)
				cancelPut(idleErr)
				closeReadCloser(source)
				_ = pw.CloseWithError(idleErr)
				return
			}
		}
	}()

	putErr := func() (err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("panic in put: %v", recovered)
			}
		}()
		return put(putCtx, pr)
	}()
	close(watchDone)
	closeReadCloser(pr)

	sourceClosed := false
	if putErr != nil {
		// Closing the pipe reader only unblocks writes to the pipe. Close the
		// source too so an in-flight source Read cannot keep this call hung.
		closeReadCloser(source)
		sourceClosed = true
	}
	copyResult := <-copyDone
	if !sourceClosed {
		closeReadCloser(source)
	}
	// The put and copy errors of a stalled stream only say that the pipe or request was closed.
	if idleErr := context.Cause(putCtx); errors.Is(idleErr, ErrStreamIdle) {
		return copyResult.bytesCopied, idleErr
	}
	if putErr != nil {
		return copyResult.bytesCopied, errors.Join(putErr, copyResult.err)
	}
	if copyResult.err != nil {
		return copyResult.bytesCopied, copyResult.err
	}
	return copyResult.bytesCopied, nil
}

type streamCopyResult struct {
	bytesCopied int64
	err         error
}

// progressWriter records when bytes last moved through it, for the idle check in StreamGetToPut.
type progressWriter struct {
	w            io.Writer
	lastProgress atomic.Int64 // UnixNano
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	if n > 0 {
		p.touch()
	}
	return n, err
}

func (p *progressWriter) touch() {
	p.lastProgress.Store(time.Now().UnixNano())
}

func (p *progressWriter) idleFor() time.Duration {
	return time.Since(time.Unix(0, p.lastProgress.Load()))
}

func copyWithExactSize(ctx context.Context, dst io.Writer, src io.Reader, expectedSize int64, buf []byte) (int64, error) {
	var copied int64
	for {
		if err := ctx.Err(); err != nil {
			return copied, err
		}
		n, readErr := src.Read(buf)
		if err := ctx.Err(); err != nil {
			return copied, err
		}
		if n > 0 {
			if copied+int64(n) > expectedSize {
				return copied, fmt.Errorf("%w: expected %d bytes, read at least %d", ErrStreamSizeMismatch, expectedSize, copied+int64(n))
			}
			written, writeErr := dst.Write(buf[:n])
			if err := ctx.Err(); err != nil {
				return copied + int64(written), errors.Join(writeErr, err)
			}
			if writeErr != nil {
				return copied + int64(written), writeErr
			}
			if written != n {
				return copied + int64(written), io.ErrShortWrite
			}
			copied += int64(n)
		}
		if readErr == io.EOF {
			if copied != expectedSize {
				return copied, fmt.Errorf("%w: expected %d bytes, got %d", ErrStreamSizeMismatch, expectedSize, copied)
			}
			return copied, nil
		}
		if readErr != nil {
			return copied, readErr
		}
	}
}

func closePipeWriter(pw *io.PipeWriter, copyErr *error) {
	if pw == nil {
		return
	}
	if *copyErr != nil {
		_ = pw.CloseWithError(*copyErr)
		return
	}
	_ = pw.Close()
}
