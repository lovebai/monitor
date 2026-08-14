package agent

import "testing"

func TestTasklistCSV(t *testing.T) {
	cases := []struct {
		line string
		img  string
		pid  string
	}{
		{`"nginx.exe","1234","Console","1","8,456 K"`, "nginx.exe", "1234"},
		{`"java.exe","99","Services","0","1,234,567 K"`, "java.exe", "99"},
		{`"svchost.exe","778","Services","0","12,345 K"`, "svchost.exe", "778"},
	}
	for _, c := range cases {
		p := tasklistCSV(c.line)
		if len(p) < 2 || p[0] != c.img || p[1] != c.pid {
			t.Fatalf("tasklistCSV(%q) = %#v, want img=%s pid=%s", c.line, p, c.img, c.pid)
		}
	}
	if tasklistCSV("not csv") != nil {
		t.Fatal("expected nil for non-CSV line")
	}
}
