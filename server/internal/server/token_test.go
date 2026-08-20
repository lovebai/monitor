package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newHandlerWithTokens(t *testing.T, perAgent map[string]string) *Handler {
	t.Helper()
	cfg := Config{AgentTokens: perAgent, DatabasePath: filepath.Join(t.TempDir(), "monitor-test.db"), OfflineAfter: time.Minute}
	h, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Close() })
	return h
}

func postReportCode(h *Handler, nodeID, token string) int {
	body := fmt.Sprintf(`{"node_id":%q,"hostname":"h1","os":{"name":"linux","architecture":"amd64"},"resources":{},"interfaces":[],"network":{}}`, nodeID)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reports", strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	h.ServeHTTP(rr, req)
	return rr.Code
}

func TestAgentTokenBinding(t *testing.T) {
	h := newHandlerWithTokens(t, map[string]string{"n1": "tok1", "n2": "tok2"})
	cases := []struct {
		node, token string
		want        int
	}{
		{"n1", "tok1", http.StatusNoContent},    // 独立 Token 匹配
		{"n2", "tok2", http.StatusNoContent},    // 独立 Token 匹配
		{"n1", "tok2", http.StatusUnauthorized}, // 拿他节点的 Token 上报 → 拒绝（身份绑定生效）
		{"n1", "wrong", http.StatusUnauthorized},
		{"n1", "", http.StatusUnauthorized},
		{"n3", "tok1", http.StatusUnauthorized}, // 未配置独立 Token → 拒绝
		{"n3", "anything", http.StatusUnauthorized},
	}
	for _, c := range cases {
		if got := postReportCode(h, c.node, c.token); got != c.want {
			t.Errorf("node=%s token=%q = %d, want %d", c.node, c.token, got, c.want)
		}
	}
}

func TestUnlistedNodeRejected(t *testing.T) {
	h := newHandlerWithTokens(t, nil)
	if got := postReportCode(h, "n1", "whatever"); got != http.StatusUnauthorized {
		t.Errorf("unlisted node = %d, want 401", got)
	}
}
