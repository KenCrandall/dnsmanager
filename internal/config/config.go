package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	HTTPAddr   string
	DataDir    string
	ConfigDir  string
	ContentDir string
	UIDistDir  string
	DBPath     string
	Version    string
}

type Layout struct {
	DataDir      string `json:"dataDir"`
	ConfigDir    string `json:"configDir"`
	ContentDir   string `json:"contentDir"`
	UIDistDir    string `json:"uiDistDir"`
	DBPath       string `json:"dbPath"`
	ManagedDir   string `json:"managedDir"`
	ManualDir    string `json:"manualDir"`
	GeneratedDir string `json:"generatedDir"`
	BackupsDir   string `json:"backupsDir"`
}

func Load() Config {
	dataDir := envOrDefault("DNSMANAGER_DATA_DIR", filepath.FromSlash("./runtime/data"))
	configDir := envOrDefault("DNSMANAGER_CONFIG_DIR", filepath.FromSlash("./runtime/config"))
	contentDir := envOrDefault("DNSMANAGER_CONTENT_DIR", filepath.FromSlash("./runtime/content"))

	return Config{
		HTTPAddr:   envOrDefault("DNSMANAGER_HTTP_ADDR", ":8080"),
		DataDir:    dataDir,
		ConfigDir:  configDir,
		ContentDir: contentDir,
		UIDistDir:  envOrDefault("DNSMANAGER_UI_DIST_DIR", filepath.FromSlash("./web/dist")),
		DBPath:     envOrDefault("DNSMANAGER_DB_PATH", filepath.Join(dataDir, "dnsmanager.db")),
		Version:    envOrDefault("DNSMANAGER_VERSION", "dev"),
	}
}

func (c Config) Layout() Layout {
	return Layout{
		DataDir:      c.DataDir,
		ConfigDir:    c.ConfigDir,
		ContentDir:   c.ContentDir,
		UIDistDir:    c.UIDistDir,
		DBPath:       c.DBPath,
		ManagedDir:   filepath.Join(c.ConfigDir, "managed"),
		ManualDir:    filepath.Join(c.ConfigDir, "manual"),
		GeneratedDir: filepath.Join(c.ConfigDir, "generated"),
		BackupsDir:   filepath.Join(c.DataDir, "backups"),
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
