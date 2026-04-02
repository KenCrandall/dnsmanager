package dns

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"dnsmanager/internal/config"
	"dnsmanager/internal/revision"

	_ "modernc.org/sqlite"
)

type Service struct {
	db        *sql.DB
	layout    config.Layout
	revisions *revision.Service
}

type Record struct {
	ID             int64     `json:"id"`
	RevisionID     int64     `json:"revisionId"`
	SourceRecordID *int64    `json:"sourceRecordId,omitempty"`
	Name           string    `json:"name"`
	RecordType     string    `json:"recordType"`
	Value          string    `json:"value"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type Workspace struct {
	Revision revision.Revision `json:"revision"`
	Records  []Record          `json:"records"`
}

type UpsertInput struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	RecordType string `json:"recordType"`
	Value      string `json:"value"`
	Summary    string `json:"summary"`
	CreatedBy  string `json:"createdBy"`
}

func New(layout config.Layout, revisions *revision.Service) (*Service, error) {
	db, err := sql.Open("sqlite", layout.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	svc := &Service{
		db:        db,
		layout:    layout,
		revisions: revisions,
	}

	if err := svc.migrate(); err != nil {
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

func (s *Service) Workspace(ctx context.Context) (Workspace, error) {
	revisionState, err := s.currentWorkspaceRevision(ctx)
	if err != nil {
		return Workspace{}, err
	}

	records, err := s.recordsForRevision(ctx, revisionState.ID)
	if err != nil {
		return Workspace{}, err
	}

	return Workspace{
		Revision: revisionState,
		Records:  records,
	}, nil
}

func (s *Service) Upsert(ctx context.Context, input UpsertInput) (Workspace, error) {
	name := normalizeName(input.Name)
	recordType := strings.ToUpper(strings.TrimSpace(input.RecordType))
	value := strings.TrimSpace(input.Value)
	if err := validateRecord(name, recordType, value); err != nil {
		return Workspace{}, err
	}

	draft, cloned, err := s.ensureDraftWorkspace(ctx, input.Summary, input.CreatedBy)
	if err != nil {
		return Workspace{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if input.ID > 0 {
		targetID, err := s.resolveRecordID(ctx, draft.ID, input.ID, cloned)
		if err != nil {
			return Workspace{}, err
		}
		_, err = s.db.ExecContext(ctx, `
			UPDATE dns_records
			SET name = ?, record_type = ?, value = ?, updated_at = ?
			WHERE id = ? AND revision_id = ?
		`, name, recordType, value, now, targetID, draft.ID)
		if err != nil {
			return Workspace{}, err
		}
	} else {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO dns_records (revision_id, name, record_type, value, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`, draft.ID, name, recordType, value, now, now)
		if err != nil {
			return Workspace{}, err
		}
	}

	return s.syncDraft(ctx, draft.ID, input.Summary, input.CreatedBy)
}

func (s *Service) Delete(ctx context.Context, recordID int64, summary, createdBy string) (Workspace, error) {
	draft, cloned, err := s.ensureDraftWorkspace(ctx, summary, createdBy)
	if err != nil {
		return Workspace{}, err
	}

	targetID, err := s.resolveRecordID(ctx, draft.ID, recordID, cloned)
	if err != nil {
		return Workspace{}, err
	}

	_, err = s.db.ExecContext(ctx, `DELETE FROM dns_records WHERE id = ? AND revision_id = ?`, targetID, draft.ID)
	if err != nil {
		return Workspace{}, err
	}

	return s.syncDraft(ctx, draft.ID, summary, createdBy)
}

func (s *Service) syncDraft(ctx context.Context, revisionID int64, summary, createdBy string) (Workspace, error) {
	records, err := s.recordsForRevision(ctx, revisionID)
	if err != nil {
		return Workspace{}, err
	}

	rendered := renderRecords(records)
	revisionState, err := s.revisions.UpdateDraft(ctx, revisionID, revision.CreateInput{
		Summary:        defaultSummary(summary, "Update managed DNS records"),
		RenderedConfig: rendered,
		CreatedBy:      defaultCreatedBy(createdBy),
	})
	if err != nil {
		return Workspace{}, err
	}

	return Workspace{Revision: revisionState, Records: records}, nil
}

func (s *Service) currentWorkspaceRevision(ctx context.Context) (revision.Revision, error) {
	draft, err := s.revisions.LatestDraft(ctx)
	if err == nil {
		return draft, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return revision.Revision{}, err
	}

	return s.revisions.Applied(ctx)
}

func (s *Service) ensureDraftWorkspace(ctx context.Context, summary, createdBy string) (revision.Revision, bool, error) {
	draft, err := s.revisions.LatestDraft(ctx)
	if err == nil {
		return draft, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return revision.Revision{}, false, err
	}

	applied, err := s.revisions.Applied(ctx)
	if err != nil {
		return revision.Revision{}, false, err
	}

	draft, err = s.revisions.CreateDraft(ctx, revision.CreateInput{
		Summary:        defaultSummary(summary, "Draft managed DNS changes"),
		RenderedConfig: applied.RenderedConfig,
		CreatedBy:      defaultCreatedBy(createdBy),
	})
	if err != nil {
		return revision.Revision{}, false, err
	}

	if err := s.cloneRecords(ctx, applied.ID, draft.ID); err != nil {
		return revision.Revision{}, false, err
	}

	return draft, true, nil
}

func (s *Service) cloneRecords(ctx context.Context, sourceRevisionID, targetRevisionID int64) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, record_type, value, created_at, updated_at
		FROM dns_records
		WHERE revision_id = ?
		ORDER BY id
	`, sourceRevisionID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id         int64
			name       string
			recordType string
			value      string
			createdAt  string
			updatedAt  string
		)
		if err := rows.Scan(&id, &name, &recordType, &value, &createdAt, &updatedAt); err != nil {
			return err
		}

		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO dns_records (
				revision_id, source_record_id, name, record_type, value, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?)
		`, targetRevisionID, id, name, recordType, value, createdAt, updatedAt); err != nil {
			return err
		}
	}

	return rows.Err()
}

func (s *Service) resolveRecordID(ctx context.Context, draftRevisionID, recordID int64, allowSourceMatch bool) (int64, error) {
	var id int64
	query := `SELECT id FROM dns_records WHERE revision_id = ? AND id = ?`
	args := []any{draftRevisionID, recordID}
	if allowSourceMatch {
		query = `SELECT id FROM dns_records WHERE revision_id = ? AND (id = ? OR source_record_id = ?) ORDER BY id LIMIT 1`
		args = []any{draftRevisionID, recordID, recordID}
	}
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("record %d was not found in the current draft workspace", recordID)
		}
		return 0, err
	}
	return id, nil
}

func (s *Service) recordsForRevision(ctx context.Context, revisionID int64) ([]Record, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, revision_id, source_record_id, name, record_type, value, created_at, updated_at
		FROM dns_records
		WHERE revision_id = ?
		ORDER BY lower(name), record_type, value, id
	`, revisionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var (
			record    Record
			sourceID  sql.NullInt64
			createdAt string
			updatedAt string
		)
		if err := rows.Scan(&record.ID, &record.RevisionID, &sourceID, &record.Name, &record.RecordType, &record.Value, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		if sourceID.Valid {
			record.SourceRecordID = &sourceID.Int64
		}
		record.CreatedAt = parseTime(createdAt)
		record.UpdatedAt = parseTime(updatedAt)
		records = append(records, record)
	}

	return records, rows.Err()
}

func (s *Service) migrate() error {
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS dns_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			revision_id INTEGER NOT NULL,
			source_record_id INTEGER,
			name TEXT NOT NULL,
			record_type TEXT NOT NULL,
			value TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return err
	}

	if _, err := s.db.Exec(`ALTER TABLE dns_records ADD COLUMN source_record_id INTEGER`); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return err
	}

	return nil
}

func renderRecords(records []Record) string {
	var builder strings.Builder
	builder.WriteString("# Managed by dnsmanager DNS editor.\n")
	if len(records) == 0 {
		builder.WriteString("# No managed DNS records are currently defined.\n")
		return builder.String()
	}

	for _, record := range records {
		builder.WriteString(renderRecord(record))
		builder.WriteString("\n")
	}

	return builder.String()
}

func validateRecord(name, recordType, value string) error {
	if name == "" {
		return errors.New("name must not be empty")
	}
	if strings.Contains(name, " ") {
		return errors.New("name must not contain spaces")
	}
	switch recordType {
	case "A":
		ip := net.ParseIP(value)
		if ip == nil || ip.To4() == nil {
			return errors.New("A records require a valid IPv4 address")
		}
	case "AAAA":
		ip := net.ParseIP(value)
		if ip == nil || ip.To4() != nil || !strings.Contains(value, ":") {
			return errors.New("AAAA records require a valid IPv6 address")
		}
	case "CNAME":
		if normalizeName(value) == "" {
			return errors.New("CNAME records require a target hostname")
		}
	case "TXT":
		if value == "" {
			return errors.New("TXT records require text content")
		}
	case "PTR":
		if value == "" {
			return errors.New("PTR records require a target hostname")
		}
	case "SRV":
		if err := validateSRVValue(value); err != nil {
			return err
		}
	default:
		return errors.New("recordType must be one of A, AAAA, CNAME, TXT, PTR, or SRV")
	}

	return nil
}

func normalizeName(name string) string {
	return strings.TrimSuffix(strings.TrimSpace(strings.ToLower(name)), ".")
}

func parseTime(raw string) time.Time {
	layouts := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func defaultSummary(summary, fallback string) string {
	summary = strings.TrimSpace(summary)
	if summary != "" {
		return summary
	}
	return fallback
}

func defaultCreatedBy(createdBy string) string {
	createdBy = strings.TrimSpace(createdBy)
	if createdBy != "" {
		return createdBy
	}
	return "cli"
}

func renderRecord(record Record) string {
	switch record.RecordType {
	case "A", "AAAA":
		return fmt.Sprintf("host-record=%s,%s", record.Name, record.Value)
	case "CNAME":
		return fmt.Sprintf("cname=%s,%s", record.Name, normalizeName(record.Value))
	case "TXT":
		return fmt.Sprintf(`txt-record=%s,"%s"`, record.Name, escapeTXT(record.Value))
	case "PTR":
		return fmt.Sprintf("ptr-record=%s,%s", record.Name, normalizeName(record.Value))
	case "SRV":
		target, port, priority, weight := parseSRVValue(record.Value)
		return fmt.Sprintf("srv-host=%s,%s,%s,%s,%s", record.Name, normalizeName(target), port, priority, weight)
	default:
		return fmt.Sprintf("# unsupported-record=%s,%s,%s", record.RecordType, record.Name, record.Value)
	}
}

func validateSRVValue(value string) error {
	target, port, priority, weight := parseSRVValue(value)
	if normalizeName(target) == "" {
		return errors.New("SRV records require a target hostname followed by port, priority, and weight")
	}

	for fieldName, raw := range map[string]string{
		"port":     port,
		"priority": priority,
		"weight":   weight,
	} {
		if raw == "" {
			return fmt.Errorf("SRV records require %s", fieldName)
		}
		if _, err := strconv.Atoi(raw); err != nil {
			return fmt.Errorf("SRV %s must be an integer", fieldName)
		}
	}

	return nil
}

func parseSRVValue(value string) (target, port, priority, weight string) {
	parts := strings.Split(value, ",")
	for len(parts) < 4 {
		parts = append(parts, "")
	}

	target = strings.TrimSpace(parts[0])
	port = strings.TrimSpace(parts[1])
	priority = strings.TrimSpace(parts[2])
	weight = strings.TrimSpace(parts[3])
	return target, port, priority, weight
}

func escapeTXT(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return value
}
