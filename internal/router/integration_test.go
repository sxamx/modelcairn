package router_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sxamx/modelcairn/internal/chatcompletions"
	"github.com/sxamx/modelcairn/internal/openaiadapter"
	"github.com/sxamx/modelcairn/internal/router"
	"github.com/sxamx/modelcairn/internal/testupstream"
)

type integrationSecrets struct{}

func (integrationSecrets) Use(ctx context.Context, name string, callback func([]byte) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if name != "fixture-secret" {
		panic("unexpected secret name")
	}
	value := []byte("x")
	defer clear(value)
	return callback(value)
}

func TestStreamingFallbackThroughOpenAIAdapterBeforeCommitment(t *testing.T) {
	valid := "data: {\"id\":\"chat-stream-1\",\"object\":\"chat.completion.chunk\",\"model\":\"physical\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"},\"finish_reason\":null}]}\n\ndata: [DONE]\n\n"
	upstream := testupstream.New(
		testupstream.Step{Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: "data: invalid\n\n"},
		testupstream.Step{Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: valid},
	)
	server := httptest.NewServer(upstream)
	defer server.Close()
	destination := func(id string) router.Destination {
		return router.Destination{ID: id, BaseURL: server.URL, Adapter: "openai-chat-v1", EgressType: "direct", AllowPrivateNetwork: true, ProviderModelID: "physical", SecretName: "fixture-secret"}
	}
	snapshot := router.Snapshot{Alias: "assistant", MaxAttempts: 2, AttemptTimeout: time.Second, TotalTimeout: 2 * time.Second, Destinations: []router.Destination{destination("one"), destination("two")}}
	parsed, err := chatcompletions.Parse([]byte(`{"model":"assistant","messages":[{"role":"user","content":"hello"}],"stream":true}`))
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	result, err := router.NewEngine(openaiadapter.New(integrationSecrets{})).RunStream(context.Background(), w, snapshot, parsed.Request)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Committed || !result.Completed || len(result.Attempts) != 2 || result.Attempts[0].FallbackReason != "invalid_response" || result.Attempts[1].Outcome != "success" {
		t.Fatalf("result=%+v", result)
	}
	if body := w.Body.String(); !strings.Contains(body, `"model":"assistant"`) || strings.Contains(body, "invalid") || !strings.HasSuffix(body, "data: [DONE]\n\n") {
		t.Fatalf("body=%q", body)
	}
	if observations := upstream.Observations(); len(observations) != 2 || !observations[0].AuthorizationPresent || !observations[1].AuthorizationPresent {
		t.Fatalf("observations=%+v", observations)
	}
}

func TestStreamingAdapterNeverSplicesAfterCommitment(t *testing.T) {
	partial := "data: {\"id\":\"chat-stream-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial\"},\"finish_reason\":null}]}\n\ndata: invalid\n\n"
	upstream := testupstream.New(
		testupstream.Step{Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: partial},
		testupstream.Step{Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: "data: [DONE]\n\n"},
	)
	server := httptest.NewServer(upstream)
	defer server.Close()
	destination := func(id string) router.Destination {
		return router.Destination{ID: id, BaseURL: server.URL, Adapter: "openai-chat-v1", EgressType: "direct", AllowPrivateNetwork: true, ProviderModelID: "physical", SecretName: "fixture-secret"}
	}
	snapshot := router.Snapshot{Alias: "assistant", MaxAttempts: 2, AttemptTimeout: time.Second, TotalTimeout: 2 * time.Second, Destinations: []router.Destination{destination("one"), destination("two")}}
	w := httptest.NewRecorder()
	result, err := router.NewEngine(openaiadapter.New(integrationSecrets{})).RunStream(context.Background(), w, snapshot, chatcompletions.Request{Model: "assistant", Stream: true})
	var runErr *router.RunError
	if !errors.As(err, &runErr) || runErr.Code != "stream_interrupted" || !result.Committed || result.Completed || len(result.Attempts) != 1 || result.Attempts[0].Outcome != "partial" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if observations := upstream.Observations(); len(observations) != 1 {
		t.Fatalf("observations=%+v", observations)
	}
}

func TestNonStreamingFallbackThroughOpenAIAdapter(t *testing.T) {
	upstream := testupstream.New(
		testupstream.Step{Status: http.StatusTooManyRequests, Header: http.Header{"Retry-After": {"3"}}},
		testupstream.Step{Header: http.Header{"Content-Type": {"application/json"}}, Body: `{"id":"chat-1","object":"chat.completion","model":"physical","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`},
	)
	server := httptest.NewServer(upstream)
	defer server.Close()
	destination := func(id string) router.Destination {
		return router.Destination{ID: id, BaseURL: server.URL, Adapter: "openai-chat-v1", EgressType: "direct", AllowPrivateNetwork: true, ProviderModelID: "physical", SecretName: "fixture-secret"}
	}
	snapshot := router.Snapshot{Alias: "assistant", MaxAttempts: 2, AttemptTimeout: time.Second, TotalTimeout: 2 * time.Second, Destinations: []router.Destination{destination("one"), destination("two")}}
	parsed, err := chatcompletions.Parse([]byte(`{"model":"assistant","messages":[{"role":"user","content":"hello"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	result, err := router.NewEngine(openaiadapter.New(integrationSecrets{})).Run(context.Background(), snapshot, parsed.Request)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Attempts) != 2 || result.Attempts[0].FallbackReason != "rate_limited" || result.Attempts[0].RetryAfter != "3" || result.Upstream.StatusCode != http.StatusOK {
		t.Fatalf("result=%+v", result)
	}
	observations := upstream.Observations()
	if len(observations) != 2 || !observations[0].AuthorizationPresent || observations[0].Path != "/chat/completions" {
		t.Fatalf("observations=%+v", observations)
	}
}

func BenchmarkStreamingRouterConcurrency(b *testing.B) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"id\":\"bench\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"},\"finish_reason\":null}]}\n\ndata: [DONE]\n\n"))
	}))
	defer upstream.Close()
	snapshot := router.Snapshot{Alias: "assistant", MaxAttempts: 1, AttemptTimeout: 2 * time.Second, TotalTimeout: 3 * time.Second,
		Destinations: []router.Destination{{ID: "one", BaseURL: upstream.URL, Adapter: "openai-chat-v1", EgressType: "direct", AllowPrivateNetwork: true, ProviderModelID: "physical", SecretName: "fixture-secret"}}}
	request := chatcompletions.Request{Model: "assistant", Stream: true}
	engine := router.NewEngine(openaiadapter.New(integrationSecrets{}))
	for _, concurrency := range []int{1, 2, 5, 10, 20} {
		b.Run(fmt.Sprintf("streams-%d", concurrency), func(b *testing.B) {
			b.ReportAllocs()
			started := time.Now()
			for iteration := 0; iteration < b.N; iteration++ {
				var group sync.WaitGroup
				errorsFound := make(chan error, concurrency)
				for worker := 0; worker < concurrency; worker++ {
					group.Add(1)
					go func() {
						defer group.Done()
						result, err := engine.RunStream(context.Background(), httptest.NewRecorder(), snapshot, request)
						if err != nil || !result.Completed {
							errorsFound <- fmt.Errorf("completed=%v: %w", result.Completed, err)
						}
					}()
				}
				group.Wait()
				close(errorsFound)
				for err := range errorsFound {
					b.Fatal(err)
				}
			}
			elapsed := time.Since(started).Seconds()
			b.ReportMetric(float64(b.N*concurrency)/elapsed, "streams/s")
		})
	}
}
