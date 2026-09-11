package storage

import (
	"context"
	"fmt"
	"testing"
)

func TestResourceAndSecretPagesUseImmutableContinuations(t *testing.T) {
	ctx := context.Background()
	i, err := OpenInstallation(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer i.Close()
	repository := NewRepository(i.DB())
	for index := 0; index < 5; index++ {
		name := fmt.Sprintf("provider-%d", index)
		if _, err := repository.Put(ctx, PutResource{Kind: KindProvider, Name: name, Spec: []byte(`{}`)}, Actor{Type: "cli"}); err != nil {
			t.Fatal(err)
		}
		if _, err := i.Secrets().Put(ctx, PutSecret{Name: fmt.Sprintf("secret-%d", index), Value: []byte("secret value")}, Actor{Type: "cli"}); err != nil {
			t.Fatal(err)
		}
	}
	resourceSeen, secretSeen := map[string]bool{}, map[string]bool{}
	resourceCursor, secretCursor := "", ""
	for {
		page, err := repository.ListPage(ctx, KindProvider, resourceCursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range page.Items {
			if resourceSeen[item.ID] {
				t.Fatal("duplicate resource across pages")
			}
			resourceSeen[item.ID] = true
		}
		resourceCursor = page.NextID
		if resourceCursor == "" {
			break
		}
	}
	for {
		page, err := i.Secrets().ListMetadataPage(ctx, secretCursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range page.Items {
			if secretSeen[item.Name] {
				t.Fatal("duplicate secret across pages")
			}
			secretSeen[item.Name] = true
		}
		secretCursor = page.NextID
		if secretCursor == "" {
			break
		}
	}
	if len(resourceSeen) != 5 || len(secretSeen) != 5 {
		t.Fatalf("resources=%d secrets=%d", len(resourceSeen), len(secretSeen))
	}
}
