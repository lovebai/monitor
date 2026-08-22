package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestServerInfoCollector(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "monitor-test.db")
	if err := os.WriteFile(dbPath, make([]byte, 4096), 0644); err != nil {
		t.Fatal(err)
	}
	si := newServerInfo(dbPath)
	st := si.stats()
	if st.Hostname == "" {
		t.Error("hostname should not be empty")
	}
	if st.OSName == "" || st.Arch == "" {
		t.Error("os/arch should not be empty")
	}
	if st.MemTotalBytes == 0 {
		t.Error("memory total should be collected")
	}
	if st.DiskTotalBytes == 0 {
		t.Error("disk total should be collected")
	}
	if st.DBFileSize < 4096 {
		t.Errorf("db file size = %d, want >= 4096", st.DBFileSize)
	}
	if st.DBPath != dbPath {
		t.Errorf("DBPath = %q, want %q", st.DBPath, dbPath)
	}
	// 刷新间隔内的第二次调用应返回相同缓存。
	st2 := si.stats()
	if st2.DBFileSize != st.DBFileSize {
		t.Error("cached stats should be stable within refresh interval")
	}
}
