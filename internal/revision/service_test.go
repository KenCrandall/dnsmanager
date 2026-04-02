package revision

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dnsmanager/internal/config"
)

func TestRevisionLifecycle(t *testing.T) {
	t.Helper()

	root := t.TempDir()
	layout := config.Layout{
		DataDir:             filepath.Join(root, "data"),
		ConfigDir:           filepath.Join(root, "config"),
		ContentDir:          filepath.Join(root, "content"),
		DBPath:              filepath.Join(root, "data", "dnsmanager.db"),
		ManagedDir:          filepath.Join(root, "config", "managed"),
		ManualDir:           filepath.Join(root, "config", "manual"),
		GeneratedDir:        filepath.Join(root, "config", "generated"),
		BackupsDir:          filepath.Join(root, "data", "backups"),
		StagingDir:          filepath.Join(root, "data", "staging"),
		AppliedDir:          filepath.Join(root, "data", "applied"),
		ActiveGeneratedFile: filepath.Join(root, "config", "generated", "00-dnsmanager-foundation.conf"),
	}

	if err := ensureTestLayout(layout); err != nil {
		t.Fatalf("ensure layout: %v", err)
	}

	svc, err := New(layout)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	defer svc.Close()

	svc.validator = func(ctx context.Context, path string) validationResult {
		return validationResult{Status: "passed", Output: "dnsmasq --test passed"}
	}

	current, err := svc.Current(context.Background())
	if err != nil {
		t.Fatalf("current bootstrap revision: %v", err)
	}
	if current.State != "applied" {
		t.Fatalf("expected bootstrap revision to be applied, got %s", current.State)
	}

	draft, err := svc.CreateDraft(context.Background(), CreateInput{
		Summary:        "add local zone",
		RenderedConfig: "address=/lab.local/192.168.10.50\n",
		CreatedBy:      "test",
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if !strings.Contains(draft.DiffText, "lab.local") {
		t.Fatalf("expected diff to mention new config, got %q", draft.DiffText)
	}

	validated, err := svc.Validate(context.Background(), draft.ID)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if validated.ValidationStatus != "passed" {
		t.Fatalf("expected passed validation, got %s", validated.ValidationStatus)
	}

	applied, err := svc.Apply(context.Background(), draft.ID)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if applied.State != "applied" {
		t.Fatalf("expected applied state, got %s", applied.State)
	}

	rolledBack, err := svc.Rollback(context.Background(), current.ID)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if rolledBack.State != "applied" {
		t.Fatalf("expected rollback revision to be applied, got %s", rolledBack.State)
	}
	if rolledBack.SourceRevisionID == nil || *rolledBack.SourceRevisionID != current.ID {
		t.Fatalf("expected rollback source revision %d, got %+v", current.ID, rolledBack.SourceRevisionID)
	}
}

func ensureTestLayout(layout config.Layout) error {
	for _, path := range []string{
		layout.DataDir,
		layout.ConfigDir,
		layout.ContentDir,
		layout.ManagedDir,
		layout.ManualDir,
		layout.GeneratedDir,
		layout.BackupsDir,
		layout.StagingDir,
		layout.AppliedDir,
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
	}

	return os.WriteFile(layout.ActiveGeneratedFile, []byte("# baseline\n"), 0o644)
}
