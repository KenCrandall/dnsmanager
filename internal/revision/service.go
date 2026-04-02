package revision

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"dnsmanager/internal/config"

	_ "modernc.org/sqlite"
)

type Service struct {
	db        *sql.DB
	layout    config.Layout
	validator validatorFunc
}

type validatorFunc func(context.Context, string) validationResult

type validationResult struct {
	Status string
	Output string
}

type Revision struct {
	ID               int64      `json:"id"`
	State            string     `json:"state"`
	Summary          string     `json:"summary"`
	RenderedConfig   string     `json:"renderedConfig"`
	DiffText         string     `json:"diffText"`
	ValidationStatus string     `json:"validationStatus"`
	ValidationOutput string     `json:"validationOutput"`
	CreatedBy        string     `json:"createdBy"`
	CreatedAt        time.Time  `json:"createdAt"`
	AppliedAt        *time.Time `json:"appliedAt,omitempty"`
	SourceRevisionID *int64     `json:"sourceRevisionId,omitempty"`
}

type CreateInput struct {
	Summary        string `json:"summary"`
	RenderedConfig string `json:"renderedConfig"`
	CreatedBy      string `json:"createdBy"`
}

func New(layout config.Layout) (*Service, error) {
	db, err := sql.Open("sqlite", layout.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	svc := &Service{
		db:        db,
		layout:    layout,
		validator: dnsmasqValidator,
	}

	if err := svc.migrate(); err != nil {
		return nil, err
	}

	if err := svc.ensureBootstrapRevision(context.Background()); err != nil {
		return nil, err
	}

	return svc, nil
}

func (s *Service) Close() error {
	if s.db == nil {
		return nil
	}

	return s.db.Close()
}

func (s *Service) List(ctx context.Context) ([]Revision, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, state, summary, rendered_config, diff_text, validation_status,
		       validation_output, created_by, created_at, applied_at, source_revision_id
		FROM config_revisions
		ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var revisions []Revision
	for rows.Next() {
		revision, scanErr := scanRevision(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		revisions = append(revisions, revision)
	}

	return revisions, rows.Err()
}

func (s *Service) Current(ctx context.Context) (Revision, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, state, summary, rendered_config, diff_text, validation_status,
		       validation_output, created_by, created_at, applied_at, source_revision_id
		FROM config_revisions
		ORDER BY CASE WHEN state = 'applied' THEN 0 ELSE 1 END, id DESC
		LIMIT 1
	`)

	return scanRevision(row)
}

func (s *Service) Applied(ctx context.Context) (Revision, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, state, summary, rendered_config, diff_text, validation_status,
		       validation_output, created_by, created_at, applied_at, source_revision_id
		FROM config_revisions
		WHERE state = 'applied'
		ORDER BY id DESC
		LIMIT 1
	`)

	return scanRevision(row)
}

func (s *Service) LatestDraft(ctx context.Context) (Revision, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, state, summary, rendered_config, diff_text, validation_status,
		       validation_output, created_by, created_at, applied_at, source_revision_id
		FROM config_revisions
		WHERE state IN ('draft', 'validated')
		ORDER BY id DESC
		LIMIT 1
	`)

	return scanRevision(row)
}

func (s *Service) Get(ctx context.Context, id int64) (Revision, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, state, summary, rendered_config, diff_text, validation_status,
		       validation_output, created_by, created_at, applied_at, source_revision_id
		FROM config_revisions
		WHERE id = ?
	`, id)

	return scanRevision(row)
}

func (s *Service) CreateDraft(ctx context.Context, input CreateInput) (Revision, error) {
	current, err := s.Applied(ctx)
	if err != nil {
		return Revision{}, err
	}

	if strings.TrimSpace(input.RenderedConfig) == "" {
		return Revision{}, errors.New("renderedConfig must not be empty")
	}

	summary := strings.TrimSpace(input.Summary)
	if summary == "" {
		summary = fmt.Sprintf("Draft created %s", time.Now().UTC().Format(time.RFC3339))
	}

	createdBy := strings.TrimSpace(input.CreatedBy)
	if createdBy == "" {
		createdBy = "cli"
	}

	createdAt := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO config_revisions (
			state, summary, rendered_config, diff_text, validation_status,
			validation_output, created_by, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"draft",
		summary,
		input.RenderedConfig,
		buildDiff(current.RenderedConfig, input.RenderedConfig),
		"pending",
		"Validation has not been run.",
		createdBy,
		createdAt,
	)
	if err != nil {
		return Revision{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Revision{}, err
	}

	return s.Get(ctx, id)
}

func (s *Service) UpdateDraft(ctx context.Context, id int64, input CreateInput) (Revision, error) {
	revision, err := s.Get(ctx, id)
	if err != nil {
		return Revision{}, err
	}

	if revision.State == "applied" || revision.State == "superseded" {
		return Revision{}, errors.New("only draft revisions can be updated")
	}

	applied, err := s.Applied(ctx)
	if err != nil {
		return Revision{}, err
	}

	if strings.TrimSpace(input.RenderedConfig) == "" {
		return Revision{}, errors.New("renderedConfig must not be empty")
	}

	summary := strings.TrimSpace(input.Summary)
	if summary == "" {
		summary = revision.Summary
	}

	createdBy := strings.TrimSpace(input.CreatedBy)
	if createdBy == "" {
		createdBy = revision.CreatedBy
	}
	if createdBy == "" {
		createdBy = "cli"
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE config_revisions
		SET state = 'draft',
		    summary = ?,
		    rendered_config = ?,
		    diff_text = ?,
		    validation_status = 'pending',
		    validation_output = 'Validation has not been run.',
		    created_by = ?
		WHERE id = ?
	`, summary, input.RenderedConfig, buildDiff(applied.RenderedConfig, input.RenderedConfig), createdBy, id)
	if err != nil {
		return Revision{}, err
	}

	return s.Get(ctx, id)
}

func (s *Service) Validate(ctx context.Context, id int64) (Revision, error) {
	revision, err := s.Get(ctx, id)
	if err != nil {
		return Revision{}, err
	}

	stageRoot, entryFile, err := s.renderStage(revision)
	if err != nil {
		return Revision{}, err
	}

	result := s.validator(ctx, entryFile)
	result.Output = strings.TrimSpace(result.Output)
	if result.Output == "" {
		result.Output = fmt.Sprintf("Validation completed for %s", stageRoot)
	}

	state := revision.State
	if result.Status == "passed" {
		state = "validated"
	} else if result.Status == "failed" {
		state = "draft"
	}

	if _, err := s.db.ExecContext(ctx, `
		UPDATE config_revisions
		SET state = ?, validation_status = ?, validation_output = ?
		WHERE id = ?
	`, state, result.Status, result.Output, id); err != nil {
		return Revision{}, err
	}

	return s.Get(ctx, id)
}

func (s *Service) Apply(ctx context.Context, id int64) (Revision, error) {
	revision, err := s.Get(ctx, id)
	if err != nil {
		return Revision{}, err
	}

	if revision.ValidationStatus == "pending" {
		revision, err = s.Validate(ctx, id)
		if err != nil {
			return Revision{}, err
		}
	}

	if revision.ValidationStatus == "failed" {
		return Revision{}, errors.New("cannot apply a revision with failed validation")
	}

	if err := s.backupCurrentGenerated(); err != nil {
		return Revision{}, err
	}

	if err := atomicWriteFile(s.layout.ActiveGeneratedFile, []byte(revision.RenderedConfig), 0o644); err != nil {
		return Revision{}, fmt.Errorf("write active generated config: %w", err)
	}

	snapshotPath := filepath.Join(s.layout.AppliedDir, fmt.Sprintf("revision-%06d.conf", revision.ID))
	if err := atomicWriteFile(snapshotPath, []byte(revision.RenderedConfig), 0o644); err != nil {
		return Revision{}, fmt.Errorf("write applied snapshot: %w", err)
	}

	appliedAt := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Revision{}, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `UPDATE config_revisions SET state = 'superseded' WHERE state = 'applied' AND id <> ?`, revision.ID); err != nil {
		return Revision{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE config_revisions
		SET state = 'applied', applied_at = ?, validation_output = TRIM(validation_output || CHAR(10) || ?)
		WHERE id = ?
	`, appliedAt, "Applied to "+s.layout.ActiveGeneratedFile, revision.ID); err != nil {
		return Revision{}, err
	}

	if err := tx.Commit(); err != nil {
		return Revision{}, err
	}

	return s.Get(ctx, id)
}

func (s *Service) Rollback(ctx context.Context, sourceID int64) (Revision, error) {
	source, err := s.Get(ctx, sourceID)
	if err != nil {
		return Revision{}, err
	}

	rollbackDraft, err := s.createRollbackDraft(ctx, source)
	if err != nil {
		return Revision{}, err
	}

	return s.Apply(ctx, rollbackDraft.ID)
}

func (s *Service) createRollbackDraft(ctx context.Context, source Revision) (Revision, error) {
	current, err := s.Current(ctx)
	if err != nil {
		return Revision{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO config_revisions (
			state, summary, rendered_config, diff_text, validation_status,
			validation_output, created_by, created_at, source_revision_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"draft",
		fmt.Sprintf("Rollback to revision #%d", source.ID),
		source.RenderedConfig,
		buildDiff(current.RenderedConfig, source.RenderedConfig),
		"pending",
		fmt.Sprintf("Rollback draft created from revision #%d", source.ID),
		"system",
		now,
		source.ID,
	)
	if err != nil {
		return Revision{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Revision{}, err
	}

	return s.Get(ctx, id)
}

func (s *Service) renderStage(revision Revision) (string, string, error) {
	stageRoot := filepath.Join(s.layout.StagingDir, fmt.Sprintf("revision-%06d", revision.ID))
	if err := os.RemoveAll(stageRoot); err != nil {
		return "", "", fmt.Errorf("reset stage root: %w", err)
	}

	manualDir := filepath.Join(stageRoot, "manual")
	managedDir := filepath.Join(stageRoot, "managed")
	generatedDir := filepath.Join(stageRoot, "generated")

	for _, path := range []string{manualDir, managedDir, generatedDir} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return "", "", err
		}
	}

	if err := copyDir(s.layout.ManualDir, manualDir); err != nil {
		return "", "", err
	}

	if err := copyDir(s.layout.ManagedDir, managedDir); err != nil {
		return "", "", err
	}

	generatedFile := filepath.Join(generatedDir, filepath.Base(s.layout.ActiveGeneratedFile))
	if err := atomicWriteFile(generatedFile, []byte(revision.RenderedConfig), 0o644); err != nil {
		return "", "", err
	}

	entryFile := filepath.Join(stageRoot, "dnsmasq.conf")
	entryConfig := fmt.Sprintf(strings.TrimSpace(`
no-daemon
bind-dynamic
log-facility=-
conf-dir=%s,*.conf
conf-dir=%s,*.conf
conf-dir=%s,*.conf
enable-tftp
tftp-root=%s
`)+"\n", manualDir, managedDir, generatedDir, s.layout.ContentDir)
	if err := atomicWriteFile(entryFile, []byte(entryConfig), 0o644); err != nil {
		return "", "", err
	}

	return stageRoot, entryFile, nil
}

func (s *Service) backupCurrentGenerated() error {
	currentBytes, err := os.ReadFile(s.layout.ActiveGeneratedFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	target := filepath.Join(s.layout.BackupsDir, fmt.Sprintf("generated-%s.conf", time.Now().UTC().Format("20060102T150405.000000000Z07")))
	return atomicWriteFile(target, currentBytes, 0o644)
}

func (s *Service) ensureBootstrapRevision(ctx context.Context) error {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM config_revisions`).Scan(&count); err != nil {
		return err
	}

	if count > 0 {
		return nil
	}

	currentConfig, err := os.ReadFile(s.layout.ActiveGeneratedFile)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO config_revisions (
			state, summary, rendered_config, diff_text, validation_status,
			validation_output, created_by, created_at, applied_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"applied",
		"Foundation bootstrap",
		string(currentConfig),
		"Initial generated config baseline.",
		"skipped",
		"Bootstrap revision created from current generated config.",
		"system",
		now,
		now,
	)
	return err
}

func (s *Service) migrate() error {
	schema, err := loadSchema()
	if err != nil {
		return err
	}

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}

	migrations := []string{
		`ALTER TABLE config_revisions ADD COLUMN rendered_config TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE config_revisions ADD COLUMN validation_status TEXT NOT NULL DEFAULT 'pending'`,
		`ALTER TABLE config_revisions ADD COLUMN validation_output TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE config_revisions ADD COLUMN source_revision_id INTEGER`,
	}

	for _, statement := range migrations {
		if _, err := s.db.Exec(statement); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return err
		}
	}

	return nil
}

func loadSchema() (string, error) {
	var candidates []string

	if _, file, _, ok := runtime.Caller(0); ok {
		candidates = append(candidates, filepath.Join(filepath.Dir(file), "..", "..", "db", "schema.sql"))
	}

	candidates = append(candidates,
		filepath.Join("db", "schema.sql"),
		filepath.Join("/app", "db", "schema.sql"),
	)

	for _, candidate := range candidates {
		content, err := os.ReadFile(candidate)
		if err == nil {
			return string(content), nil
		}
	}

	return "", errors.New("could not locate schema.sql")
}

func dnsmasqValidator(ctx context.Context, entryFile string) validationResult {
	if _, err := exec.LookPath("dnsmasq"); err != nil {
		return validationResult{
			Status: "skipped",
			Output: "dnsmasq binary is not available; validation skipped.",
		}
	}

	cmd := exec.CommandContext(ctx, "dnsmasq", "--test", "--conf-file="+entryFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return validationResult{
			Status: "failed",
			Output: strings.TrimSpace(string(output) + "\n" + err.Error()),
		}
	}

	return validationResult{
		Status: "passed",
		Output: strings.TrimSpace(string(output)),
	}
}

func scanRevision(scanner interface {
	Scan(dest ...any) error
}) (Revision, error) {
	var (
		revision       Revision
		createdAt      string
		appliedAt      sql.NullString
		sourceRevision sql.NullInt64
	)

	err := scanner.Scan(
		&revision.ID,
		&revision.State,
		&revision.Summary,
		&revision.RenderedConfig,
		&revision.DiffText,
		&revision.ValidationStatus,
		&revision.ValidationOutput,
		&revision.CreatedBy,
		&createdAt,
		&appliedAt,
		&sourceRevision,
	)
	if err != nil {
		return Revision{}, err
	}

	revision.CreatedAt = mustParseTime(createdAt)
	if appliedAt.Valid {
		parsed := mustParseTime(appliedAt.String)
		revision.AppliedAt = &parsed
	}
	if sourceRevision.Valid {
		revision.SourceRevisionID = &sourceRevision.Int64
	}

	return revision, nil
}

func mustParseTime(raw string) time.Time {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
	}

	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.UTC()
		}
	}

	return time.Time{}
}

func buildDiff(current, next string) string {
	if current == next {
		return "No changes."
	}

	var builder strings.Builder
	builder.WriteString("--- current\n")
	builder.WriteString("+++ proposed\n")

	if current != "" {
		for _, line := range strings.Split(strings.TrimSuffix(current, "\n"), "\n") {
			builder.WriteString("- ")
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}

	if next != "" {
		for _, line := range strings.Split(strings.TrimSuffix(next, "\n"), "\n") {
			builder.WriteString("+ ")
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}

	return strings.TrimSpace(builder.String())
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		sourcePath := filepath.Join(src, entry.Name())
		targetPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
			if err := copyDir(sourcePath, targetPath); err != nil {
				return err
			}
			continue
		}

		content, err := os.ReadFile(sourcePath)
		if err != nil {
			return err
		}
		if err := atomicWriteFile(targetPath, content, 0o644); err != nil {
			return err
		}
	}

	return nil
}

func atomicWriteFile(path string, content []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	buffer := bytes.NewBuffer(content)
	if err := os.WriteFile(tmpPath, buffer.Bytes(), mode); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}
