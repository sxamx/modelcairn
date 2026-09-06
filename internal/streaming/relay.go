// Package streaming contains the bounded Phase 1 streaming contract spike.
package streaming

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
)

const readBufferSize = 32 * 1024

var (
	ErrEmptyUpstream        = errors.New("upstream stream ended before commitment")
	ErrStreamingUnsupported = errors.New("response writer does not support streaming")
)

type Result struct {
	Committed bool
	Bytes     int64
}

// RelaySSE copies a validated upstream SSE body without transforming bytes. It
// reads upstream before sending headers, so an immediate failure stays eligible
// for caller-controlled fallback. Any error after commitment must end the stream.
func RelaySSE(ctx context.Context, destination http.ResponseWriter, upstream io.ReadCloser) (Result, error) {
	var closeOnce sync.Once
	closeUpstream := func() { closeOnce.Do(func() { _ = upstream.Close() }) }
	stopClose := context.AfterFunc(ctx, closeUpstream)
	defer func() {
		stopClose()
		closeUpstream()
	}()

	flusher, ok := destination.(http.Flusher)
	if !ok {
		return Result{}, ErrStreamingUnsupported
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	buffer := make([]byte, readBufferSize)
	read, readErr := upstream.Read(buffer)
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if read == 0 {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		if readErr == nil {
			readErr = io.ErrNoProgress
		} else if errors.Is(readErr, io.EOF) {
			readErr = ErrEmptyUpstream
		}
		return Result{}, fmt.Errorf("read before stream commitment: %w", readErr)
	}

	destination.Header().Set("Content-Type", "text/event-stream")
	destination.Header().Set("Cache-Control", "no-cache")
	destination.Header().Set("X-Accel-Buffering", "no")
	destination.WriteHeader(http.StatusOK)
	result := Result{Committed: true}
	if err := writeAndFlush(destination, flusher, buffer[:read], &result); err != nil {
		return result, err
	}
	if readErr != nil {
		if errors.Is(readErr, io.EOF) {
			return result, nil
		}
		return result, fmt.Errorf("read committed stream: %w", readErr)
	}

	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		read, readErr = upstream.Read(buffer)
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if read > 0 {
			if err := writeAndFlush(destination, flusher, buffer[:read], &result); err != nil {
				return result, err
			}
		}
		if errors.Is(readErr, io.EOF) {
			return result, nil
		}
		if readErr != nil {
			return result, fmt.Errorf("read committed stream: %w", readErr)
		}
		if read == 0 {
			return result, fmt.Errorf("read committed stream: %w", io.ErrNoProgress)
		}
	}
}

func writeAndFlush(destination http.ResponseWriter, flusher http.Flusher, data []byte, result *Result) error {
	written, err := destination.Write(data)
	result.Bytes += int64(written)
	flusher.Flush()
	if err != nil {
		return fmt.Errorf("write committed stream: %w", err)
	}
	if written != len(data) {
		return io.ErrShortWrite
	}
	return nil
}
