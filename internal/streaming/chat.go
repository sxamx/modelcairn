package streaming

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
)

const maxSSEEventBytes = 1 << 20

var (
	ErrInvalidSSE       = errors.New("invalid upstream SSE")
	ErrIncompleteStream = errors.New("upstream SSE ended without DONE")
)

type ChatResult struct {
	Committed bool
	Completed bool
	Bytes     int64
	Events    int
}

func RelayChatCompletions(ctx context.Context, destination http.ResponseWriter, upstream io.ReadCloser, requestedAlias string) (ChatResult, error) {
	var closeOnce sync.Once
	closeUpstream := func() { closeOnce.Do(func() { _ = upstream.Close() }) }
	stopClose := context.AfterFunc(ctx, closeUpstream)
	defer func() { stopClose(); closeUpstream() }()
	flusher, ok := destination.(http.Flusher)
	if !ok {
		return ChatResult{}, ErrStreamingUnsupported
	}
	reader := bufio.NewReaderSize(upstream, readBufferSize)
	result := ChatResult{}
	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		payload, err := readSSEData(reader)
		if contextErr := ctx.Err(); contextErr != nil {
			return result, contextErr
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				if !result.Committed {
					return result, ErrEmptyUpstream
				}
				return result, ErrIncompleteStream
			}
			return result, err
		}
		if payload == nil {
			continue
		}
		frame := []byte("data: ")
		if bytes.Equal(payload, []byte("[DONE]")) {
			frame = append(frame, "[DONE]\n\n"...)
			if !result.Committed {
				return result, ErrInvalidSSE
			}
			if err := writeChatFrame(destination, flusher, frame, &result); err != nil {
				return result, err
			}
			result.Completed = true
			return result, nil
		}
		normalized, err := normalizeChunk(payload, requestedAlias)
		if err != nil {
			return result, fmt.Errorf("normalize SSE chunk: %w", err)
		}
		frame = append(frame, normalized...)
		frame = append(frame, '\n', '\n')
		if !result.Committed {
			destination.Header().Set("Content-Type", "text/event-stream")
			destination.Header().Set("Cache-Control", "no-cache")
			destination.Header().Set("X-Accel-Buffering", "no")
			destination.WriteHeader(http.StatusOK)
			result.Committed = true
		}
		if err := writeChatFrame(destination, flusher, frame, &result); err != nil {
			return result, err
		}
		result.Events++
	}
}

func readSSEData(reader *bufio.Reader) ([]byte, error) {
	var event, line []byte
	for {
		fragment, prefix, err := reader.ReadLine()
		if len(line)+len(fragment) > maxSSEEventBytes {
			return nil, ErrInvalidSSE
		}
		line = append(line, fragment...)
		if prefix {
			continue
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		if len(line) == 0 {
			if len(event) > 0 {
				return bytes.TrimSuffix(event, []byte{'\n'}), nil
			}
			if errors.Is(err, io.EOF) {
				return nil, io.EOF
			}
			line = line[:0]
			continue
		}
		if line[0] != ':' {
			field, value, found := bytes.Cut(line, []byte{':'})
			if found && bytes.Equal(field, []byte("data")) {
				value = bytes.TrimPrefix(value, []byte{' '})
				if len(event)+len(value)+1 > maxSSEEventBytes {
					return nil, ErrInvalidSSE
				}
				event = append(event, value...)
				event = append(event, '\n')
			}
		}
		line = line[:0]
		if errors.Is(err, io.EOF) {
			if len(event) > 0 {
				return bytes.TrimSuffix(event, []byte{'\n'}), nil
			}
			return nil, io.EOF
		}
	}
}

func normalizeChunk(payload []byte, alias string) ([]byte, error) {
	var object map[string]json.RawMessage
	if json.Unmarshal(payload, &object) != nil || object == nil {
		return nil, ErrInvalidSSE
	}
	var id, kind string
	var choices []json.RawMessage
	if json.Unmarshal(object["id"], &id) != nil || id == "" || json.Unmarshal(object["object"], &kind) != nil || kind != "chat.completion.chunk" || json.Unmarshal(object["choices"], &choices) != nil || choices == nil {
		return nil, ErrInvalidSSE
	}
	for _, rawChoice := range choices {
		var choice struct {
			Index        *int                       `json:"index"`
			Delta        map[string]json.RawMessage `json:"delta"`
			FinishReason json.RawMessage            `json:"finish_reason"`
		}
		if json.Unmarshal(rawChoice, &choice) != nil || choice.Index == nil || *choice.Index < 0 || choice.Delta == nil {
			return nil, ErrInvalidSSE
		}
		if roleRaw, exists := choice.Delta["role"]; exists {
			var role string
			if json.Unmarshal(roleRaw, &role) != nil || role != "assistant" {
				return nil, ErrInvalidSSE
			}
		}
		if content, exists := choice.Delta["content"]; exists && string(content) != "null" {
			var text string
			if json.Unmarshal(content, &text) != nil {
				return nil, ErrInvalidSSE
			}
		}
		if calls, exists := choice.Delta["tool_calls"]; exists {
			var values []json.RawMessage
			if json.Unmarshal(calls, &values) != nil || values == nil {
				return nil, ErrInvalidSSE
			}
		}
		if len(choice.FinishReason) > 0 && string(choice.FinishReason) != "null" {
			var reason string
			if json.Unmarshal(choice.FinishReason, &reason) != nil {
				return nil, ErrInvalidSSE
			}
		}
	}
	encodedAlias, _ := json.Marshal(alias)
	object["model"] = encodedAlias
	return json.Marshal(object)
}

func writeChatFrame(destination http.ResponseWriter, flusher http.Flusher, frame []byte, result *ChatResult) error {
	written, err := destination.Write(frame)
	result.Bytes += int64(written)
	flusher.Flush()
	if err != nil {
		return err
	}
	if written != len(frame) {
		return io.ErrShortWrite
	}
	return nil
}
