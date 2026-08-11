package mail

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"
)

func TestBuild_PlainTextOnly(t *testing.T) {
	msg := Message{From: "a@x.com", To: []string{"b@x.com"}, Subject: "hi", Text: "hello world"}
	raw, err := build(msg)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	m, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("parse built message: %v", err)
	}
	if got := m.Header.Get("Subject"); got != "hi" {
		t.Errorf("Subject = %q", got)
	}
	ct := m.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
	body, _ := io.ReadAll(m.Body)
	if !strings.Contains(string(body), "hello world") {
		t.Errorf("body = %q", body)
	}
}

func TestBuild_BccNotInHeaders(t *testing.T) {
	msg := Message{From: "a@x.com", To: []string{"b@x.com"}, Bcc: []string{"secret@x.com"}, Text: "x"}
	raw, err := build(msg)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if strings.Contains(string(raw), "secret@x.com") {
		t.Error("Bcc address leaked into message headers/body -- must never appear there")
	}
}

func TestBuild_TextAndHTML_Alternative(t *testing.T) {
	msg := Message{From: "a@x.com", To: []string{"b@x.com"}, Text: "plain version", HTML: "<b>html version</b>"}
	raw, err := build(msg)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	parts := parseAllParts(t, raw)
	if len(parts) != 2 {
		t.Fatalf("got %d parts, want 2 (text/plain + text/html)", len(parts))
	}
	if !strings.HasPrefix(parts[0].contentType, "text/plain") {
		t.Errorf("part 0 = %s, want text/plain first (RFC 2046: least-preferred first)", parts[0].contentType)
	}
	if !strings.HasPrefix(parts[1].contentType, "text/html") {
		t.Errorf("part 1 = %s, want text/html last", parts[1].contentType)
	}
	if !strings.Contains(parts[0].body, "plain version") {
		t.Errorf("plain part body = %q", parts[0].body)
	}
	if !strings.Contains(parts[1].body, "html version") {
		t.Errorf("html part body = %q", parts[1].body)
	}
}

func TestBuild_InlineImage_ContentIDPresent(t *testing.T) {
	msg := Message{
		From: "a@x.com", To: []string{"b@x.com"},
		HTML: `<html><body><img src="cid:logo1"></body></html>`,
		Attachments: []Attachment{
			{Filename: "logo.png", Data: []byte{0x89, 'P', 'N', 'G'}, ContentType: "image/png", Inline: true, CID: "logo1"},
		},
	}
	raw, err := build(msg)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	// Content-ID gets canonicalized to "Content-Id" by textproto
	// (standard, case-insensitive per MIME) -- check case-insensitively
	// rather than assuming a specific casing survives.
	if !strings.Contains(strings.ToLower(string(raw)), "content-id: <logo1>") {
		t.Error("Content-ID header for inline image not found")
	}
	if !strings.Contains(string(raw), `cid:logo1`) {
		t.Error("HTML body should still reference cid:logo1")
	}
	if !strings.Contains(string(raw), "multipart/related") {
		t.Error("expected a multipart/related wrapper for inline image + HTML")
	}
}

func TestBuild_InlineImage_MissingCID_Errors(t *testing.T) {
	msg := Message{
		From: "a@x.com", To: []string{"b@x.com"}, HTML: "<b>x</b>",
		Attachments: []Attachment{{Filename: "x.png", Data: []byte{1}, Inline: true}}, // no CID
	}
	if _, err := build(msg); err == nil {
		t.Fatal("expected error for inline attachment with no CID")
	}
}

func TestBuild_RegularAttachment(t *testing.T) {
	msg := Message{
		From: "a@x.com", To: []string{"b@x.com"}, Text: "see attached",
		Attachments: []Attachment{{Filename: "report.pdf", Data: []byte("PDFDATA"), ContentType: "application/pdf"}},
	}
	raw, err := build(msg)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.Contains(string(raw), "multipart/mixed") {
		t.Error("expected multipart/mixed for a message with a regular attachment")
	}
	if !strings.Contains(string(raw), `filename="report.pdf"`) {
		t.Error("attachment filename not found")
	}
	if !strings.Contains(string(raw), "Content-Disposition: attachment") {
		t.Error("expected Content-Disposition: attachment for a non-inline file")
	}
}

func TestBareAddr(t *testing.T) {
	cases := map[string]string{
		"a@x.com":                  "a@x.com",
		"Name <a@x.com>":           "a@x.com",
		"\"Full Name\" <a@x.com>":  "a@x.com",
	}
	for in, want := range cases {
		if got := bareAddr(in); got != want {
			t.Errorf("bareAddr(%q) = %q, want %q", in, got, want)
		}
	}
}

// --- test helpers -----------------------------------------------------

type parsedPart struct {
	contentType string
	body        string
}

// parseAllParts parses raw as a top-level (possibly multipart) message
// and returns every leaf part it finds, recursing into nested
// multiparts one level deep (enough for these tests -- none of them
// build more than 2 levels of nesting).
func parseAllParts(t *testing.T, raw []byte) []parsedPart {
	t.Helper()
	m, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("parse message: %v", err)
	}
	ct := m.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(ct)
	if err != nil {
		t.Fatalf("parse content-type %q: %v", ct, err)
	}
	if !strings.HasPrefix(mediaType, "multipart/") {
		body, _ := io.ReadAll(m.Body)
		return []parsedPart{{contentType: ct, body: string(body)}}
	}
	mr := multipart.NewReader(m.Body, params["boundary"])
	var out []parsedPart
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("next part: %v", err)
		}
		body, _ := io.ReadAll(p)
		out = append(out, parsedPart{contentType: p.Header.Get("Content-Type"), body: string(body)})
	}
	return out
}
