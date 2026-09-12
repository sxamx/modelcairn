package chatcompletions

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDataOpenAPITracksExecutableContract(t *testing.T) {
	raw, err := os.ReadFile("../../docs/contratos/api/data-v1.openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(raw, &document); err != nil {
		t.Fatalf("parse OpenAPI: %v", err)
	}
	if document["openapi"] != "3.1.0" {
		t.Fatalf("openapi=%v", document["openapi"])
	}
	paths := mustMap(t, document, "paths")
	endpoint := mustMap(t, paths, "/v1/chat/completions")
	post := mustMap(t, endpoint, "post")
	if post["operationId"] != "createChatCompletion" {
		t.Fatalf("operationId=%v", post["operationId"])
	}
	components := mustMap(t, document, "components")
	schemas := mustMap(t, components, "schemas")
	request := mustMap(t, schemas, "ChatCompletionRequest")
	if request["additionalProperties"] != false {
		t.Fatal("request schema must reject unknown fields")
	}
	properties := mustMap(t, request, "properties")
	model := mustMap(t, properties, "model")
	if model["maxLength"] != MaxRequestAliasBytes {
		t.Fatalf("model maxLength=%v parser=%v", model["maxLength"], MaxRequestAliasBytes)
	}
	messages := mustMap(t, properties, "messages")
	if messages["maxItems"] != MaxMessages {
		t.Fatalf("messages maxItems=%v parser=%v", messages["maxItems"], MaxMessages)
	}
	tools := mustMap(t, properties, "tools")
	if tools["maxItems"] != MaxTools {
		t.Fatalf("tools maxItems=%v parser=%v", tools["maxItems"], MaxTools)
	}
}

func mustMap(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := parent[key].(map[string]any)
	if !ok {
		t.Fatalf("%s is not an object: %#v", key, parent[key])
	}
	return value
}
