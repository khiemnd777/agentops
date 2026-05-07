package config

import (
	"net"
	"net/url"
	"os"
	"strings"
	"time"
)

type Config struct {
	APIAddr       string
	DatabaseURL   string
	AuthEnabled   bool
	MCPToken      string
	PublicBaseURL string
	MCPPublicURL  string
	RunImport     RunImportConfig
}

type RunImportConfig struct {
	AutoImportOnReviewOpen   bool
	AutoImportOnTaskListOpen bool
	DebounceSeconds          int
	MaxReportsPerScan        int
	ReportGlob               string
	StalePolicy              string
	UpdatePolicy             string
}

func Load() Config {
	return Config{
		APIAddr:       env("API_ADDR", ":8080"),
		DatabaseURL:   databaseURL(),
		AuthEnabled:   strings.EqualFold(env("AUTH_ENABLED", "false"), "true"),
		MCPToken:      env("AGENTOPS_MCP_TOKEN", ""),
		PublicBaseURL: publicBaseURL(),
		MCPPublicURL:  mcpPublicURL(),
		RunImport: RunImportConfig{
			AutoImportOnReviewOpen:   true,
			AutoImportOnTaskListOpen: true,
			DebounceSeconds:          30,
			MaxReportsPerScan:        100,
			ReportGlob:               ".codex/reports/runs/*/run.report.json",
			StalePolicy:              "show_last_valid",
			UpdatePolicy:             "import_new_revision",
		},
	}
}

func mcpPublicURL() string {
	if v := env("AGENTOPS_MCP_PUBLIC_URL", ""); v != "" {
		return v
	}
	return strings.TrimRight(publicBaseURL(), "/") + "/mcp"
}

func publicBaseURL() string {
	if v := env("AGENTOPS_PUBLIC_BASE_URL", ""); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "http://localhost:" + env("API_HOST_PORT", env("API_CONTAINER_PORT", "8080"))
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func databaseURL() string {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v
	}

	host := env("POSTGRES_HOST", "localhost")
	port := postgresPort(host)
	dbname := env("POSTGRES_DBNAME", "agentops")
	user := env("POSTGRES_USER", "agentops")
	password := env("POSTGRES_PASSWORD", "agentops")

	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   net.JoinHostPort(host, port),
		Path:   "/" + dbname,
	}
	q := u.Query()
	q.Set("sslmode", "disable")
	u.RawQuery = q.Encode()
	return u.String()
}

func postgresPort(host string) string {
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return env("POSTGRES_HOST_PORT", env("POSTGRES_CONTAINER_PORT", "5432"))
	}
	return env("POSTGRES_CONTAINER_PORT", "5432")
}

func (c Config) ImportDebounce() time.Duration {
	return time.Duration(c.RunImport.DebounceSeconds) * time.Second
}
