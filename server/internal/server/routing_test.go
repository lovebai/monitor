package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServeMuxRouting(t *testing.T) {
	h := newTestHandlerAuth(t, false)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/nope", nil))
	if rr.Code != http.StatusNotFound {
		t.Errorf("unknown path = %d, want 404", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/reports", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/v1/reports = %d, want 405", rr.Code)
	}

	body := `{"node_id":"n1","hostname":"h1","os":{"name":"linux","architecture":"amd64"},"resources":{},"interfaces":[],"network":{}}`
	rr = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reports", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer t")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("ingest = %d, want 204", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/nodes/n1/history", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "[") {
		t.Errorf("history = %d body=%q, want 200 JSON array", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/nodes/n1", nil))
	if rr.Code != http.StatusOK {
		t.Errorf("detail = %d, want 200", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/nodes/n1/extra", nil))
	if rr.Code != http.StatusNotFound {
		t.Errorf("nested detail path = %d, want 404", rr.Code)
	}
}

func TestServeMuxPprofOnlyInDebug(t *testing.T) {
	h := newTestHandlerAuth(t, false)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil))
	if rr.Code != http.StatusNotFound {
		t.Errorf("pprof without debug = %d, want 404", rr.Code)
	}

	cfg := Config{AgentTokens: map[string]string{"n1": "t"}, DatabasePath: filepath.Join(t.TempDir(), "monitor-test.db"), OfflineAfter: time.Minute, Debug: true}
	hd, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { hd.Close() })
	rr = httptest.NewRecorder()
	hd.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil))
	if rr.Code != http.StatusOK {
		t.Errorf("pprof with debug = %d, want 200", rr.Code)
	}
}
