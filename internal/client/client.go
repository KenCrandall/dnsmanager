package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"dnsmanager/internal/revision"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

type StatusResponse struct {
	Service   string    `json:"service"`
	Version   string    `json:"version"`
	HTTPAddr  string    `json:"httpAddr"`
	StartedAt time.Time `json:"startedAt"`
	Paths     struct {
		DataDir      string `json:"dataDir"`
		ConfigDir    string `json:"configDir"`
		ContentDir   string `json:"contentDir"`
		UIDistDir    string `json:"uiDistDir"`
		DBPath       string `json:"dbPath"`
		ManagedDir   string `json:"managedDir"`
		ManualDir    string `json:"manualDir"`
		GeneratedDir string `json:"generatedDir"`
		BackupsDir   string `json:"backupsDir"`
	} `json:"paths"`
}

func New(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *Client) Status(ctx context.Context) (StatusResponse, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/status", nil)
	if err != nil {
		return StatusResponse{}, err
	}

	var status StatusResponse
	if err := c.do(req, &status); err != nil {
		return StatusResponse{}, err
	}

	return status, nil
}

func (c *Client) ListRevisions(ctx context.Context) ([]revision.Revision, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/config/revisions", nil)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Revisions []revision.Revision `json:"revisions"`
	}
	if err := c.do(req, &payload); err != nil {
		return nil, err
	}

	return payload.Revisions, nil
}

func (c *Client) CurrentRevision(ctx context.Context) (revision.Revision, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/config/revisions/current", nil)
	if err != nil {
		return revision.Revision{}, err
	}

	var current revision.Revision
	if err := c.do(req, &current); err != nil {
		return revision.Revision{}, err
	}

	return current, nil
}

func (c *Client) CreateDraft(ctx context.Context, input revision.CreateInput) (revision.Revision, error) {
	req, err := c.newRequest(ctx, http.MethodPost, "/api/v1/config/revisions", input)
	if err != nil {
		return revision.Revision{}, err
	}

	var created revision.Revision
	if err := c.do(req, &created); err != nil {
		return revision.Revision{}, err
	}

	return created, nil
}

func (c *Client) ValidateRevision(ctx context.Context, id int64) (revision.Revision, error) {
	return c.revisionAction(ctx, id, "validate")
}

func (c *Client) ApplyRevision(ctx context.Context, id int64) (revision.Revision, error) {
	return c.revisionAction(ctx, id, "apply")
}

func (c *Client) RollbackRevision(ctx context.Context, id int64) (revision.Revision, error) {
	return c.revisionAction(ctx, id, "rollback")
}

func (c *Client) revisionAction(ctx context.Context, id int64, action string) (revision.Revision, error) {
	req, err := c.newRequest(ctx, http.MethodPost, fmt.Sprintf("/api/v1/config/revisions/%d/%s", id, action), map[string]any{})
	if err != nil {
		return revision.Revision{}, err
	}

	var updated revision.Revision
	if err := c.do(req, &updated); err != nil {
		return revision.Revision{}, err
	}

	return updated, nil
}

func (c *Client) newRequest(ctx context.Context, method, path string, payload any) (*http.Request, error) {
	var body io.Reader
	if payload != nil {
		buffer := bytes.NewBuffer(nil)
		if err := json.NewEncoder(buffer).Encode(payload); err != nil {
			return nil, err
		}
		body = buffer
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}

	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	return req, nil
}

func (c *Client) do(req *http.Request, dest any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = resp.Status
		}
		return fmt.Errorf("request failed: %s", message)
	}

	if dest == nil {
		return nil
	}

	return json.NewDecoder(resp.Body).Decode(dest)
}
