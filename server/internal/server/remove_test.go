package server

import (
	"path/filepath"
	"testing"
	"time"
)

func TestRemoveNode(t *testing.T) {
	h, err := New(Config{AgentTokens: map[string]string{"n1": "t"}, DatabasePath: filepath.Join(t.TempDir(), "monitor-test.db"), OfflineAfter: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	now := time.Now().UTC().Unix()
	seed := []struct {
		q    string
		args []any
	}{
		{`INSERT INTO nodes(node_id,hostname,last_seen,report_json) VALUES('n1','h1',?,'{}')`, []any{now}},
		{`INSERT INTO metrics(node_id,collected_at,cpu,memory_percent,disk_percent,latency_ms,rx_rate,tx_rate) VALUES('n1',?,1,1,1,1,1,1)`, []any{now}},
		{`INSERT INTO alerts(node_id,kind,message,active,created_at,updated_at) VALUES('n1','offline','x',1,?,?)`, []any{now, now}},
	}
	for _, s := range seed {
		if _, err := h.db.Exec(s.q, s.args...); err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}
	ok, err := h.NodeExists("n1")
	if err != nil || !ok {
		t.Fatalf("NodeExists = %v, %v, want true", ok, err)
	}
	if err := h.RemoveNode("n1"); err != nil {
		t.Fatal(err)
	}
	ok, _ = h.NodeExists("n1")
	if ok {
		t.Error("node should be removed")
	}
	var nodes, metrics, alerts int
	if err := h.db.QueryRow(`SELECT (SELECT COUNT(*) FROM nodes),(SELECT COUNT(*) FROM metrics),(SELECT COUNT(*) FROM alerts)`).Scan(&nodes, &metrics, &alerts); err != nil {
		t.Fatal(err)
	}
	if nodes != 0 || metrics != 0 || alerts != 0 {
		t.Errorf("leftover rows: nodes=%d metrics=%d alerts=%d", nodes, metrics, alerts)
	}
}

func TestPruneMetricsRetention(t *testing.T) {
	h, err := New(Config{AgentTokens: map[string]string{"n1": "t"}, DatabasePath: filepath.Join(t.TempDir(), "monitor-test.db"), HistoryRetentionDays: 30})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	now := time.Now().UTC().Unix()
	old := now - 40*86400
	for _, ts := range []int64{old, now} {
		if _, err := h.db.Exec(`INSERT INTO metrics(node_id,collected_at,cpu,memory_percent,disk_percent,latency_ms,rx_rate,tx_rate) VALUES('n1',?,1,1,1,1,1,1)`, ts); err != nil {
			t.Fatal(err)
		}
	}
	h.pruneMetrics(time.Now())
	var n int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM metrics`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("metrics rows after prune = %d, want 1 (只保留 30 天内)", n)
	}
}
