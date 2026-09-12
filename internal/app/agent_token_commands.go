package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/sxamx/modelcairn/internal/storage"
)

func runAgentToken(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "agent-token requires status, issue, or revoke")
		return 2
	}
	command := args[0]
	if command != "status" && command != "issue" && command != "revoke" {
		fmt.Fprintln(stderr, "unknown agent-token command")
		return 2
	}
	flags, server, sessionFile := onlineAdminFlags("agent-token " + command)
	if flags.Parse(args[1:]) != nil || flags.NArg() != 1 || *server == "" {
		fmt.Fprintf(stderr, "usage: modelcairn agent-token %s --server origin [--session-file path] <name>\n", command)
		return 2
	}
	client, path, err := onlineAdminClient(*server, *sessionFile)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	session, err := loadOnlineSession(path, client.server)
	if err != nil {
		return writeCLIError(stderr, err)
	}
	method := http.MethodPost
	endpoint := "/api/v1/admin/agent-tokens/" + url.PathEscape(flags.Arg(0)) + "/" + command
	if command == "status" {
		method = http.MethodGet
	}
	response, data, err := client.authenticatedWithRecovery(ctx, &session, path, method, endpoint, nil, "")
	if err != nil {
		return writeCLIError(stderr, err)
	}
	switch command {
	case "status":
		if response.StatusCode != http.StatusOK {
			return writeCLIError(stderr, fmt.Errorf("agent_token_http_%d", response.StatusCode))
		}
		var value struct {
			TokenStatus storage.AgentTokenStatus `json:"tokenStatus"`
		}
		if json.Unmarshal(data, &value) != nil || value.TokenStatus.State == "" {
			return writeCLIError(stderr, fmt.Errorf("invalid_admin_response"))
		}
		return encodeCLIJSON(stdout, stderr, value)
	case "issue":
		if response.StatusCode != http.StatusCreated {
			return writeCLIError(stderr, fmt.Errorf("agent_token_http_%d", response.StatusCode))
		}
		var value storage.IssuedAgentToken
		if json.Unmarshal(data, &value) != nil || value.Token == "" {
			return writeCLIError(stderr, fmt.Errorf("invalid_admin_response"))
		}
		// This is the sole intentional bearer delivery boundary.
		return encodeCLIJSON(stdout, stderr, value)
	default:
		if response.StatusCode != http.StatusNoContent {
			return writeCLIError(stderr, fmt.Errorf("agent_token_http_%d", response.StatusCode))
		}
		fmt.Fprintln(stdout, "agent token revoked")
		return 0
	}
}

func encodeCLIJSON(stdout, stderr io.Writer, value any) int {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return writeCLIError(stderr, err)
	}
	return 0
}
