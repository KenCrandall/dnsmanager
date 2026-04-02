package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/status", nil)
	if err != nil {
		return StatusResponse{}, err
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return StatusResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return StatusResponse{}, fmt.Errorf("unexpected status %s", resp.Status)
	}

	var status StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return StatusResponse{}, err
	}

	return status, nil
}
