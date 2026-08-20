package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFileConfigHistoryRetention(t *testing.T) {
	p := filepath.Join(t.TempDir(), "server.yaml")
	if err := os.WriteFile(p, []byte("token: t\nhistory_retention_days: 7\n"), 0644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadFileConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.HistoryRetentionDays != 7 {
		t.Errorf("HistoryRetentionDays = %d, want 7", c.HistoryRetentionDays)
	}
	if got := c.Runtime().HistoryRetentionDays; got != 7 {
		t.Errorf("Runtime HistoryRetentionDays = %d, want 7", got)
	}
}

func TestLoadFileConfigDefaultRetention(t *testing.T) {
	p := filepath.Join(t.TempDir(), "server.yaml")
	if err := os.WriteFile(p, []byte("token: t\n"), 0644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadFileConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.HistoryRetentionDays != 30 {
		t.Errorf("default HistoryRetentionDays = %d, want 30", c.HistoryRetentionDays)
	}
}

func TestLoadFileConfigAuth(t *testing.T) {
	p := filepath.Join(t.TempDir(), "server.yaml")
	if err := os.WriteFile(p, []byte("token: t\nauth_enabled: true\nauth_username: admin\nauth_password: secret\n"), 0644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadFileConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if !c.AuthEnabled || c.AuthUsername != "admin" || c.AuthPassword != "secret" {
		t.Errorf("auth config not parsed: %+v", c)
	}
}

func TestLoadFileConfigAuthMissingCredentials(t *testing.T) {
	p := filepath.Join(t.TempDir(), "server.yaml")
	if err := os.WriteFile(p, []byte("token: t\nauth_enabled: true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFileConfig(p); err == nil {
		t.Fatal("auth_enabled without credentials should fail")
	}
}

func TestLoadFileConfigAgentTokens(t *testing.T) {
	p := filepath.Join(t.TempDir(), "server.yaml")
	if err := os.WriteFile(p, []byte("token: global\nagent_tokens:\n  web-01: abc123\n  web-02: def456\n"), 0644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadFileConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.AgentTokens["web-01"] != "abc123" || c.AgentTokens["web-02"] != "def456" || len(c.AgentTokens) != 2 {
		t.Errorf("AgentTokens = %v", c.AgentTokens)
	}
	if got := c.Runtime().AgentTokens; got["web-01"] != "abc123" {
		t.Errorf("Runtime AgentTokens = %v", got)
	}
}
