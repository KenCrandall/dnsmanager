package dns

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dnsmanager/internal/config"
	"dnsmanager/internal/revision"
)

func TestDNSWorkspaceCreatesAndUpdatesDraft(t *testing.T) {
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

	if err := ensureDNSLayout(layout); err != nil {
		t.Fatalf("ensure layout: %v", err)
	}

	revisions, err := revision.New(layout)
	if err != nil {
		t.Fatalf("new revision service: %v", err)
	}
	defer revisions.Close()

	dnsService, err := New(layout, revisions)
	if err != nil {
		t.Fatalf("new dns service: %v", err)
	}
	defer dnsService.Close()

	workspace, err := dnsService.Upsert(context.Background(), UpsertInput{
		Name:       "lab.local",
		RecordType: "A",
		Value:      "192.168.10.50",
		Summary:    "Add lab record",
		CreatedBy:  "test",
	})
	if err != nil {
		t.Fatalf("upsert record: %v", err)
	}

	if workspace.Revision.State != "draft" {
		t.Fatalf("expected draft revision, got %s", workspace.Revision.State)
	}
	if len(workspace.Records) != 1 {
		t.Fatalf("expected one record, got %d", len(workspace.Records))
	}
	if !strings.Contains(workspace.Revision.RenderedConfig, "host-record=lab.local,192.168.10.50") {
		t.Fatalf("expected rendered config to include host-record line, got %q", workspace.Revision.RenderedConfig)
	}

	recordID := workspace.Records[0].ID
	workspace, err = dnsService.Upsert(context.Background(), UpsertInput{
		ID:         recordID,
		Name:       "lab.local",
		RecordType: "A",
		Value:      "192.168.10.60",
		Summary:    "Update lab record",
		CreatedBy:  "test",
	})
	if err != nil {
		t.Fatalf("update record: %v", err)
	}
	if !strings.Contains(workspace.Revision.RenderedConfig, "192.168.10.60") {
		t.Fatalf("expected updated rendered config, got %q", workspace.Revision.RenderedConfig)
	}

	workspace, err = dnsService.Delete(context.Background(), recordID, "Remove lab record", "test")
	if err != nil {
		t.Fatalf("delete record: %v", err)
	}
	if len(workspace.Records) != 0 {
		t.Fatalf("expected zero records after delete, got %d", len(workspace.Records))
	}
}

func ensureDNSLayout(layout config.Layout) error {
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
