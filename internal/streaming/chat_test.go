package streaming

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func validChunk(model, content string) string {
	return `{"id":"chunk-1","object":"chat.completion.chunk","model":"` + model + `","choices":[{"index":0,"delta":{"role":"assistant","content":"` + content + `"},"finish_reason":null}]}`
}

func TestRelayChatCompletionsValidatesNormalizesAndCompletes(t *testing.T) {
	stream := ": keepalive\n\ndata: " + validChunk("physical", "hello") + "\n\n" +
		"data: {\"id\":\"chunk-1\",\"object\":\"chat.completion.chunk\",\"model\":\"physical\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: {\"id\":\"chunk-1\",\"object\":\"chat.completion.chunk\",\"model\":\"physical\",\"choices\":[],\"usage\":{\"prompt_tokens\":9,\"completion_tokens\":3}}\n\n" +
		"data: [DONE]\n\n"
	w := &recordingWriter{}
	result, err := RelayChatCompletions(context.Background(), w, io.NopCloser(strings.NewReader(stream)), "assistant")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Committed || !result.Completed || result.Events != 3 || w.flushes != 4 || result.FirstEventAt.IsZero() ||
		result.InputTokens == nil || *result.InputTokens != 9 || result.OutputTokens == nil || *result.OutputTokens != 3 {
		t.Fatalf("result=%+v flushes=%d", result, w.flushes)
	}
	if strings.Contains(w.body.String(), "physical") || strings.Count(w.body.String(), `"model":"assistant"`) != 3 || !strings.HasSuffix(w.body.String(), "data: [DONE]\n\n") {
		t.Fatalf("body=%q", w.body.String())
	}
}

func TestRelayChatCompletionsRejectsInvalidFirstEventBeforeCommit(t *testing.T) {
	for _, stream := range []string{
		"data: not-json\n\n",
		"data: [DONE]\n\n",
		"data: {\"id\":\"x\",\"object\":\"wrong\",\"choices\":[]}\n\n",
		"data: {\"id\":\"x\",\"id\":\"y\",\"object\":\"chat.completion.chunk\",\"choices\":[]}\n\n",
	} {
		w := &recordingWriter{}
		result, err := RelayChatCompletions(context.Background(), w, io.NopCloser(strings.NewReader(stream)), "assistant")
		if err == nil || result.Committed || w.status != 0 || w.body.Len() != 0 {
			t.Fatalf("stream=%q result=%+v status=%d err=%v", stream, result, w.status, err)
		}
	}
}

func TestRelayChatCompletionsFailureAfterCommitIsPartial(t *testing.T) {
	stream := "data: " + validChunk("physical", "hello") + "\n\ndata: invalid\n\n"
	w := &recordingWriter{}
	result, err := RelayChatCompletions(context.Background(), w, io.NopCloser(strings.NewReader(stream)), "assistant")
	if err == nil || !result.Committed || result.Completed || result.Events != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if !strings.Contains(w.body.String(), "assistant") || strings.Contains(w.body.String(), "invalid") {
		t.Fatalf("body=%q", w.body.String())
	}
}

func TestRelayChatCompletionsRequiresDone(t *testing.T) {
	stream := "data: " + validChunk("physical", "hello") + "\n\n"
	result, err := RelayChatCompletions(context.Background(), &recordingWriter{}, io.NopCloser(strings.NewReader(stream)), "assistant")
	if !errors.Is(err, ErrIncompleteStream) || !result.Committed || result.Completed {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestRelayChatCompletionsAcceptsMultilineDataAndToolDelta(t *testing.T) {
	payload := `{"id":"chunk-1","object":"chat.completion.chunk","model":"physical",` + "\n" + `"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{}"}}]},"finish_reason":null}]}`
	stream := "data: " + strings.Replace(payload, "\n", "\ndata: ", 1) + "\n\ndata: [DONE]\n\n"
	w := &recordingWriter{}
	result, err := RelayChatCompletions(context.Background(), w, io.NopCloser(strings.NewReader(stream)), "assistant")
	if err != nil || !result.Completed {
		t.Fatalf("result=%+v err=%v body=%q", result, err, w.body.String())
	}
}

func TestRelayChatCompletionsBoundsEventAndClosesOnCancellation(t *testing.T) {
	oversized := "data: " + strings.Repeat("x", maxSSEEventBytes+1) + "\n\n"
	result, err := RelayChatCompletions(context.Background(), &recordingWriter{}, io.NopCloser(strings.NewReader(oversized)), "assistant")
	if !errors.Is(err, ErrInvalidSSE) || result.Committed {
		t.Fatalf("result=%+v err=%v", result, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	reader := &blockingReadCloser{started: make(chan struct{}), closed: make(chan struct{})}
	resultCh, errCh := make(chan ChatResult, 1), make(chan error, 1)
	go func() {
		got, relayErr := RelayChatCompletions(ctx, &recordingWriter{}, reader, "assistant")
		resultCh <- got
		errCh <- relayErr
	}()
	<-reader.started
	cancel()
	if got := <-resultCh; got.Committed {
		t.Fatalf("result=%+v", got)
	}
	if relayErr := <-errCh; !errors.Is(relayErr, context.Canceled) {
		t.Fatalf("err=%v", relayErr)
	}
}

func TestReadSSEDataSkipsCommentsAndEmptyEvents(t *testing.T) {
	reader := bufio.NewReader(bytes.NewBufferString(": comment\n\n\ndata: x\n\n"))
	payload, err := readSSEData(reader)
	if err != nil || string(payload) != "x" {
		t.Fatalf("payload=%q err=%v", payload, err)
	}
}

func TestRelayChatCompletionsRejectsMalformedToolCallBeforeCommit(t *testing.T) {
	for _, calls := range []string{`[null]`, `[{}]`, `[{"index":0,"function":{}}]`, `[{"index":-1,"function":{"arguments":"{}"}}]`} {
		t.Run(calls, func(t *testing.T) {
			body := `data: {"id":"chunk-1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"tool_calls":` + calls + `},"finish_reason":null}]}\n\n`
			response := httptest.NewRecorder()
			result, err := RelayChatCompletions(context.Background(), response, io.NopCloser(strings.NewReader(body)), "alias")
			if !errors.Is(err, ErrInvalidSSE) || result.Committed || response.Body.Len() != 0 {
				t.Fatalf("result=%+v body=%q err=%v", result, response.Body.String(), err)
			}
		})
	}
}
