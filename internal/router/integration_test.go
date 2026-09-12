package router_test

import (
	"context"
	"net/http"
	"net/http/httptest"
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
