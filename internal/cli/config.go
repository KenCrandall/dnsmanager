package cli

import "os"

func DefaultServerURL() string {
	if value := os.Getenv("DNSMANAGER_SERVER"); value != "" {
		return value
	}

	return "http://127.0.0.1:8080"
}
