package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTruncate_ShortTextUnchanged(t *testing.T) {
	s := "short message"
	if got := truncate(s, 100); got != s {
		t.Errorf("truncate returned %q, want unchanged %q", got, s)
	}
}

func TestTruncate_LongTextCutWithMarker(t *testing.T) {
	long := strings.Repeat("x", 3000)
	got := truncate(long, 2000)
	if len(got) > 2000 {
		t.Errorf("truncated length = %d, want <= 2000", len(got))
	}
	if !strings.Contains(got, "truncated") {
		t.Error("truncated text should contain a visible truncation marker")
	}
	if !strings.Contains(got, "3000") {
		t.Error("truncation marker should state the original total length")
	}
}

func TestTruncate_ExactBoundary(t *testing.T) {
	s := strings.Repeat("y", 500)
	if got := truncate(s, 500); got != s {
		t.Error("text exactly at the limit should not be modified")
	}
}

// --- confirm every provider actually applies its own limit ------------

func TestDiscord_Send_TruncatesLongText(t *testing.T) {
	var gotLen int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		decodeJSON(t, r, &body)
		gotLen = len(body["content"])
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	d := Discord{WebhookURL: srv.URL}
	err := d.Send(context.Background(), Message{Text: strings.Repeat("a", 5000)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotLen > MaxLengthDiscord {
		t.Errorf("sent content length = %d, want <= %d (Discord's hard limit)", gotLen, MaxLengthDiscord)
	}
}

func TestTelegram_Send_TruncatesLongText(t *testing.T) {
	var gotLen int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		decodeJSON(t, r, &body)
		gotLen = len(body["text"])
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	tg := Telegram{BotToken: "x", ChatID: "1", BaseURL: srv.URL}
	err := tg.Send(context.Background(), Message{Text: strings.Repeat("b", 6000)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotLen > MaxLengthTelegram {
		t.Errorf("sent text length = %d, want <= %d (Telegram's hard limit)", gotLen, MaxLengthTelegram)
	}
}

func TestNtfy_Send_TruncatesLongText(t *testing.T) {
	var gotLen int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var buf [8192]byte
		n, _ := r.Body.Read(buf[:])
		gotLen = n
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := Ntfy{URL: srv.URL}
	err := n.Send(context.Background(), Message{Text: strings.Repeat("c", 5000)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotLen > MaxLengthNtfy {
		t.Errorf("sent body length = %d, want <= %d", gotLen, MaxLengthNtfy)
	}
}

func decodeJSON(t *testing.T, r *http.Request, v any) {
	t.Helper()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
}
