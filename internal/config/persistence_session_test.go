package config

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sxamx/modelcairn/internal/adminsettings"
	"github.com/sxamx/modelcairn/internal/storage"
)

func TestManagerApplySessionReauthorizesWithoutConsumingPlan(t *testing.T) {
	ctx := context.Background()
	i, err := storage.OpenInstallation(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer i.Close()
	spec := adminsettings.Defaults()
	spec.PublicOrigin = "http://127.0.0.1:8080"
	admin, _, err := storage.BootstrapAdmin(ctx, i, "owner", []byte("a secure password"), spec)
	if err != nil {
		t.Fatal(err)
	}
	credentials, err := storage.CreateAdminSession(ctx, i, storage.VerifiedAdmin{ID: admin.ID, Username: admin.Username, AuthVersion: admin.AuthVersion}, 1800, 3600, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	doc := mustParse(t, basePrefix+"resources:\n- {kind: Provider, metadata: {name: provider}, spec: {}}")
	manager := NewManager(i)
	plan, err := manager.Plan(ctx, doc, false)
	if err != nil {
		t.Fatal(err)
	}
	if plan.ExpiresAt.IsZero() {
		t.Fatal("plan expiry missing")
	}
	reset, err := storage.ResetAdminPassword(ctx, i, []byte("another secure password"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ApplySession(ctx, plan.Token, doc, false, credentials.SessionToken, credentials.CSRFToken); !errors.Is(err, storage.ErrAdminSessionInvalid) {
		t.Fatalf("stale session=%v", err)
	}
	var consumed int
	if err := i.DB().QueryRow("SELECT count(*) FROM consumed_plan_tokens").Scan(&consumed); err != nil {
		t.Fatal(err)
	}
	if consumed != 0 {
		t.Fatal("authorization failure consumed plan")
	}
	fresh, err := storage.CreateAdminSession(ctx, i, storage.VerifiedAdmin{ID: reset.ID, Username: reset.Username, AuthVersion: reset.AuthVersion}, 1800, 3600, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	result, err := manager.ApplySession(ctx, plan.Token, doc, false, fresh.SessionToken, fresh.CSRFToken)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Changes) != 1 || result.Changes[0].Action != "create" || result.AppliedAt.IsZero() {
		t.Fatalf("result=%+v", result)
	}
}
