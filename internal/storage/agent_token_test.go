package storage

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func agentTokenFixture(t *testing.T) (*Installation, *AgentTokenService, AdminSessionCredentials, map[ResourceKind]Resource) {
	t.Helper()
	i, admin := sessionFixture(t)
	items := seedRepositoryGraph(t, NewRepository(i.DB()), i)
	session, err := CreateAdminSession(context.Background(), i, VerifiedAdmin{ID: admin.ID, Username: admin.Username, AuthVersion: admin.AuthVersion}, 1800, 3600, time.Now())
	if err != nil {
		i.Close()
		t.Fatal(err)
	}
	service, err := NewAgentTokenService(i)
	if err != nil {
		i.Close()
		t.Fatal(err)
	}
	return i, service, session, items
}

func TestAgentTokenIssueAuthenticateAndRevokeLifecycle(t *testing.T) {
	ctx := context.Background()
	i, service, session, items := agentTokenFixture(t)
	defer i.Close()
	issued, err := service.IssueSession(ctx, "agent", session.SessionToken, session.CSRFToken)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(issued.Token, agentTokenPrefix) || issued.Status.State != "active" || issued.Status.Prefix == nil || !strings.HasPrefix(issued.Token, *issued.Status.Prefix) {
		t.Fatalf("issued=%+v", issued)
	}
	var stored []byte
	if err := i.DB().QueryRow("SELECT verifier_sha256 FROM agent_tokens WHERE resource_id=?", items[KindAgentToken].ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256([]byte(issued.Token))
	if len(stored) != sha256.Size || string(stored) != string(want[:]) || string(stored) == issued.Token {
		t.Fatal("invalid verifier persistence")
	}
	identity, err := service.Authenticate(ctx, issued.Token, items[KindRoute].ID)
	if err != nil || identity.ResourceID != items[KindAgentToken].ID {
		t.Fatalf("identity=%+v err=%v", identity, err)
	}
	aliasIdentity, routeID, err := service.AuthenticateAlias(ctx, issued.Token, "assistant")
	if err != nil || aliasIdentity.ResourceID != identity.ResourceID || routeID != items[KindRoute].ID {
		t.Fatalf("alias identity=%+v route=%q err=%v", aliasIdentity, routeID, err)
	}
	if _, _, err := service.AuthenticateAlias(ctx, issued.Token, "missing"); !IsRepositoryCode(err, CodeNotFound) {
		t.Fatalf("missing alias=%v", err)
	}
	if _, _, err := service.AuthenticateAlias(ctx, "invalid", "missing"); !errors.Is(err, ErrAgentTokenInvalid) {
		t.Fatalf("invalid bearer leaked alias state: %v", err)
	}
	if _, err := service.Authenticate(ctx, issued.Token, "another-route"); !errors.Is(err, ErrAgentRouteForbidden) {
		t.Fatalf("forbidden=%v", err)
	}
	if _, err := service.IssueSession(ctx, "agent", session.SessionToken, session.CSRFToken); !IsRepositoryCode(err, CodeAlreadyExists) {
		t.Fatalf("second issue=%v", err)
	}
	if err := service.RevokeSession(ctx, "agent", session.SessionToken, session.CSRFToken); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, issued.Token, items[KindRoute].ID); !errors.Is(err, ErrAgentTokenInvalid) {
		t.Fatalf("revoked auth=%v", err)
	}
	status, err := service.Status(ctx, "agent")
	if err != nil || status.State != "revoked" || status.RevokedAt == nil {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	if err := service.RevokeSession(ctx, "agent", session.SessionToken, session.CSRFToken); err != nil {
		t.Fatal(err)
	}
	var revocations int
	if err := i.DB().QueryRow("SELECT count(*) FROM audit_events WHERE action='agent_token.revoke'").Scan(&revocations); err != nil || revocations != 1 {
		t.Fatalf("revocations=%d err=%v", revocations, err)
	}
}

func TestAgentTokenConcurrentIssueHasOneWinner(t *testing.T) {
	i, service, session, _ := agentTokenFixture(t)
	defer i.Close()
	var wait sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := service.IssueSession(context.Background(), "agent", session.SessionToken, session.CSRFToken)
			errs <- err
		}()
	}
	wait.Wait()
	close(errs)
	var successes, conflicts int
	for err := range errs {
		if err == nil {
			successes++
		} else if IsRepositoryCode(err, CodeAlreadyExists) {
			conflicts++
		} else {
			t.Fatalf("issue=%v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
}

func TestAgentTokenIssueRollsBackWithAudit(t *testing.T) {
	i, service, session, items := agentTokenFixture(t)
	defer i.Close()
	if _, err := i.DB().Exec(`CREATE TRIGGER reject_agent_issue_audit BEFORE INSERT ON audit_events WHEN NEW.action='agent_token.issue' BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.IssueSession(context.Background(), "agent", session.SessionToken, session.CSRFToken); err == nil {
		t.Fatal("issue succeeded despite audit failure")
	}
	var verifier []byte
	var issuedAt *string
	if err := i.DB().QueryRow("SELECT verifier_sha256,issued_at FROM agent_tokens WHERE resource_id=?", items[KindAgentToken].ID).Scan(&verifier, &issuedAt); err != nil {
		t.Fatal(err)
	}
	if verifier != nil || issuedAt != nil {
		t.Fatal("credential persisted without audit")
	}
}
