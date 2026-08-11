package chat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscord_Send(t *testing.T) {
	var gotBody map[string]string
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusNoContent) // Discord returns 204 on success
	}))
	defer srv.Close()

	d := Discord{WebhookURL: srv.URL + "/webhooks/123/abc"}
	err := d.Send(context.Background(), Message{Text: "hello", Title: "Alert"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/webhooks/123/abc" {
		t.Errorf("path = %q", gotPath)
	}
	if !strings.Contains(gotBody["content"], "hello") || !strings.Contains(gotBody["content"], "Alert") {
		t.Errorf("content = %q, want to contain title+text", gotBody["content"])
	}
}

func TestDiscord_Send_ErrorSurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"message":"rate limited","retry_after":1.2}`))
	}))
	defer srv.Close()

	d := Discord{WebhookURL: srv.URL}
	err := d.Send(context.Background(), Message{Text: "x"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("error = %v, want it to surface Discord's own error body", err)
	}
}

func TestSlack_Send(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	s := Slack{WebhookURL: srv.URL}
	if err := s.Send(context.Background(), Message{Text: "deploy done"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["text"] != "deploy done" {
		t.Errorf("text = %q", gotBody["text"])
	}
}

func TestNtfy_Send(t *testing.T) {
	var gotBody, gotTitle, gotPriority string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		gotTitle = r.Header.Get("Title")
		gotPriority = r.Header.Get("Priority")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := Ntfy{URL: srv.URL + "/my-topic"}
	err := n.Send(context.Background(), Message{Text: "disk full", Title: "Warning", Priority: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody != "disk full" {
		t.Errorf("body = %q, want raw text (no JSON envelope)", gotBody)
	}
	if gotTitle != "Warning" {
		t.Errorf("Title header = %q", gotTitle)
	}
	if gotPriority != "5" {
		t.Errorf("Priority header = %q", gotPriority)
	}
}

func TestNtfy_Send_NoPriorityHeaderWhenZero(t *testing.T) {
	sawHeader := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawHeader = r.Header.Get("Priority") != ""
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := Ntfy{URL: srv.URL}
	if err := n.Send(context.Background(), Message{Text: "x"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sawHeader {
		t.Error("Priority header should be absent when Priority==0 (provider default)")
	}
}

func TestGotify_Send(t *testing.T) {
	var gotBody map[string]any
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	g := Gotify{BaseURL: srv.URL + "/", Token: "app-token-123"}
	err := g.Send(context.Background(), Message{Text: "backup ok", Title: "Status", Priority: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotQuery != "token=app-token-123" {
		t.Errorf("query = %q", gotQuery)
	}
	if gotBody["message"] != "backup ok" || gotBody["title"] != "Status" {
		t.Errorf("body = %+v", gotBody)
	}
}

func TestGotify_Send_DefaultTitleWhenEmpty(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	g := Gotify{BaseURL: srv.URL, Token: "t"}
	if err := g.Send(context.Background(), Message{Text: "x"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["title"] == "" || gotBody["title"] == nil {
		t.Error("Gotify requires a non-empty title -- expected a default to be supplied")
	}
}

func TestTelegram_Send(t *testing.T) {
	var gotBody map[string]string
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	tg := Telegram{BotToken: "12345:abcXYZ", ChatID: "6789", BaseURL: srv.URL}
	err := tg.Send(context.Background(), Message{Text: "job finished", Title: "Done"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/bot12345:abcXYZ/sendMessage" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody["chat_id"] != "6789" {
		t.Errorf("chat_id = %q", gotBody["chat_id"])
	}
	if !strings.Contains(gotBody["text"], "job finished") || !strings.Contains(gotBody["text"], "Done") {
		t.Errorf("text = %q", gotBody["text"])
	}
}

func TestTelegram_Send_ErrorSurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"ok":false,"description":"chat not found"}`))
	}))
	defer srv.Close()

	tg := Telegram{BotToken: "x", ChatID: "bad", BaseURL: srv.URL}
	err := tg.Send(context.Background(), Message{Text: "x"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "chat not found") {
		t.Errorf("error = %v, want it to surface Telegram's own error body", err)
	}
}
