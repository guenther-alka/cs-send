package main

import (
	"fmt"
	"os"
	"time"
)

// logLine appends one timestamped line to path (created if needed,
// always appended -- never truncated, since cs-send is a one-shot CLI
// invoked repeatedly, typically from cron/a napp-it CS job, and each
// run's outcome should accumulate into a running history rather than
// overwrite the previous run's). A logfile write failure is reported to
// stderr but never aborts the actual send -- the mail/chat message
// already went out (or didn't) by the time this runs; a broken log path
// shouldn't mask that real result or turn into its own fatal error.
func logLine(path string, format string, args ...any) {
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not open --logfile %s: %v\n", path, err)
		return
	}
	defer f.Close()
	ts := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(f, "%s  "+format+"\n", append([]any{ts}, args...)...)
}
