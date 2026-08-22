package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFileConfigHistoryRetention(t *testing.T) {
	p := filepath.Join(t.TempDir(), "server.yaml")
	if err := os.WriteFile(p, []byte("agent_tokens:\n  n1: t\nhistory_retention_days: 7\n"), 0644); err != nil {
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
	if err := os.WriteFile(p, []byte("agent_tokens:\n  n1: t\n"), 0644); err != nil {
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
	if err := os.WriteFile(p, []byte("agent_tokens:\n  n1: t\nauth_enabled: true\nauth_username: admin\nauth_password: secret\n"), 0644); err != nil {
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
	if err := os.WriteFile(p, []byte("agent_tokens:\n  n1: t\nauth_enabled: true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFileConfig(p); err == nil {
		t.Fatal("auth_enabled without credentials should fail")
	}
}

func TestLoadFileConfigAgentTokens(t *testing.T) {
	p := filepath.Join(t.TempDir(), "server.yaml")
	if err := os.WriteFile(p, []byte("agent_tokens:\n  web-01: abc123\n  web-02: def456\n"), 0644); err != nil {
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

func TestLoadFileConfigRequiresAgentTokens(t *testing.T) {
	p := filepath.Join(t.TempDir(), "server.yaml")
	if err := os.WriteFile(p, []byte("listen: :8080\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFileConfig(p); err == nil {
		t.Fatal("server config without agent_tokens should fail")
	}
}

func TestUpdateConfigPassword(t *testing.T) {
	p := filepath.Join(t.TempDir(), "server.yaml")
	content := "listen: :8080\nauth_username: admin\nauth_password: 123456\nagent_tokens:\n  n1: t\n"
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	hash, err := GeneratePasswordHash("newpass")
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateConfigPassword(p, hash); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	s := string(b)
	if !strings.Contains(s, "auth_password: \""+hash+"\"") {
		t.Errorf("auth_password not replaced with hash: %s", s)
	}
	if !strings.Contains(s, "auth_username: admin") || !strings.Contains(s, "n1: t") {
		t.Errorf("unrelated config lines must be preserved: %s", s)
	}
	c, err := LoadFileConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.AuthPassword != hash {
		t.Errorf("loaded AuthPassword = %q, want %q", c.AuthPassword, hash)
	}
}

func TestUpdateConfigPasswordAppendsWhenMissing(t *testing.T) {
	p := filepath.Join(t.TempDir(), "server.yaml")
	if err := os.WriteFile(p, []byte("agent_tokens:\n  n1: t\n"), 0644); err != nil {
		t.Fatal(err)
	}
	hash, _ := GeneratePasswordHash("newpass")
	if err := UpdateConfigPassword(p, hash); err != nil {
		t.Fatal(err)
	}
	c, err := LoadFileConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.AuthPassword != hash {
		t.Errorf("appended AuthPassword = %q, want %q", c.AuthPassword, hash)
	}
}
