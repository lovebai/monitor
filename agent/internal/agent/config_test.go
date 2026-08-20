package agent

import "testing"

func TestParseSimpleYAMLAlias(t *testing.T) {
	c := parseSimpleYAML("server_url: http://127.0.0.1:8080\ntoken: t\nnode_id: n1\nalias: 生产服务器\ngroup: web\n")
	if c.Alias != "生产服务器" {
		t.Errorf("alias = %q, want %q", c.Alias, "生产服务器")
	}
}
