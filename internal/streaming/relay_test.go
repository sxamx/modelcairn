package streaming

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
)

type recordingWriter struct {
	header                      http.Header
	status, flushes, writeLimit int
	body                        bytes.Buffer
	writeErr                    error
}

func (w *recordingWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}
func (w *recordingWriter) WriteHeader(status int) { w.status = status }
func (w *recordingWriter) Write(data []byte) (int, error) {
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	if w.writeLimit > 0 && len(data) > w.writeLimit {
		return w.body.Write(data[:w.writeLimit])
	}
	return w.body.Write(data)
}
func (w *recordingWriter) Flush() { w.flushes++ }

type sequenceReader struct {
	reads []struct {
		data string
		err  error
	}
}

func (r *sequenceReader) Read(target []byte) (int, error) {
	if len(r.reads) == 0 {
		return 0, io.EOF
	}
	next := r.reads[0]
	r.reads = r.reads[1:]
	return copy(target, next.data), next.err
}
func (r *sequenceReader) Close() error { return nil }

type blockingReadCloser struct {
	started, closed chan struct{}
	once            sync.Once
	closes          atomic.Int32
}

func (r *blockingReadCloser) Read([]byte) (int, error) {
	r.once.Do(func() { close(r.started) })
	<-r.closed
	return 0, errors.New("reader closed")
}
func (r *blockingReadCloser) Close() error {
	r.closes.Add(1)
	select {
	case <-r.closed:
	default:
		close(r.closed)
	}
	return nil
}

type countingReadCloser struct {
	io.Reader
	closes atomic.Int32
}

func (r *countingReadCloser) Close() error { r.closes.Add(1); return nil }

func TestRelayPreservesSSEBytes(t *testing.T) {
	stream := "data: {\"id\":1}\n\ndata: [DONE]\n\n"
	w := &recordingWriter{}
	result, err := RelaySSE(context.Background(), w, io.NopCloser(bytes.NewBufferString(stream)))
	if err != nil {
		t.Fatalf("RelaySSE() error = %v", err)
	}
	if !result.Committed || result.Bytes != int64(len(stream)) {
		t.Fatalf("result = %+v", result)
	}
	if w.status != http.StatusOK || w.body.String() != stream || w.flushes == 0 {
		t.Fatalf("status=%d body=%q flushes=%d", w.status, w.body.String(), w.flushes)
	}
}

func TestRelayFailureBeforeCommitmentAllowsFallback(t *testing.T) {
	w := &recordingWriter{}
	reader := &sequenceReader{reads: []struct {
		data string
		err  error
	}{{err: io.ErrUnexpectedEOF}}}
	result, err := RelaySSE(context.Background(), w, reader)
	if err == nil || result.Committed {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	if w.status != 0 || w.body.Len() != 0 {
		t.Fatalf("destination committed: status=%d body=%q", w.status, w.body.String())
	}
}

func TestRelayFailureAfterCommitmentEndsStream(t *testing.T) {
	failure := errors.New("upstream disconnected")
	reader := &sequenceReader{reads: []struct {
		data string
		err  error
	}{{data: "data: first\n\n"}, {err: failure}}}
	w := &recordingWriter{}
	result, err := RelaySSE(context.Background(), w, reader)
	if err == nil || !errors.Is(err, failure) || !result.Committed {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	if w.body.String() != "data: first\n\n" {
		t.Fatalf("body=%q", w.body.String())
	}
}

func TestRelayCancellationBeforeCommitment(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w := &recordingWriter{}
	result, err := RelaySSE(ctx, w, io.NopCloser(bytes.NewBufferString("data: ignored\n\n")))
	if !errors.Is(err, context.Canceled) || result.Committed || w.status != 0 {
		t.Fatalf("result=%+v status=%d error=%v", result, w.status, err)
	}
}

func TestRelayCancellationInterruptsBlockedRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reader := &blockingReadCloser{started: make(chan struct{}), closed: make(chan struct{})}
	resultCh := make(chan Result, 1)
	errCh := make(chan error, 1)
	go func() { result, err := RelaySSE(ctx, &recordingWriter{}, reader); resultCh <- result; errCh <- err }()
	<-reader.started
	cancel()
	if result := <-resultCh; result.Committed {
		t.Fatalf("result=%+v", result)
	}
	if err := <-errCh; !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	if got := reader.closes.Load(); got != 1 {
		t.Fatalf("Close() calls=%d, want 1", got)
	}
}

func TestRelayClientDisconnectAndShortWrite(t *testing.T) {
	t.Run("disconnect", func(t *testing.T) {
		failure := errors.New("client disconnected")
		w := &recordingWriter{writeErr: failure}
		result, err := RelaySSE(context.Background(), w, io.NopCloser(bytes.NewBufferString("data: x\n\n")))
		if !result.Committed || !errors.Is(err, failure) {
			t.Fatalf("result=%+v error=%v", result, err)
		}
	})
	t.Run("short write", func(t *testing.T) {
		w := &recordingWriter{writeLimit: 2}
		result, err := RelaySSE(context.Background(), w, io.NopCloser(bytes.NewBufferString("data: x\n\n")))
		if !result.Committed || !errors.Is(err, io.ErrShortWrite) {
			t.Fatalf("result=%+v error=%v", result, err)
		}
	})
}

func TestRelayRequiresFlusher(t *testing.T) {
	w := struct{ http.ResponseWriter }{}
	reader := &countingReadCloser{Reader: bytes.NewBufferString("data: x\n\n")}
	result, err := RelaySSE(context.Background(), w, reader)
	if !errors.Is(err, ErrStreamingUnsupported) || result.Committed {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	if got := reader.closes.Load(); got != 1 {
		t.Fatalf("Close() calls=%d, want 1", got)
	}
}
