package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	logLine(path, "mail sent to=%s subject=%q", "a@b.com", "Test")
	logLine(path, "chat FAILED provider=%s: %v", "discord", "http 429")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read logfile: %v", err)
	}
	content := string(data)
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2 (append behavior): %q", len(lines), content)
	}
	if !strings.Contains(lines[0], "mail sent to=a@b.com") {
		t.Errorf("line 0 = %q", lines[0])
	}
	if !strings.Contains(lines[1], "chat FAILED provider=discord: http 429") {
		t.Errorf("line 1 = %q", lines[1])
	}
	// every line starts with a timestamp "YYYY-MM-DD HH:MM:SS  "
	for i, l := range lines {
		if len(l) < 20 || l[4] != '-' || l[7] != '-' {
			t.Errorf("line %d missing expected timestamp prefix: %q", i, l)
		}
	}
}

func TestLogLine_EmptyPathIsNoop(t *testing.T) {
	// must not panic or create anything when path is ""
	logLine("", "should not be written")
}
