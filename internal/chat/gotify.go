package chat

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// Gotify sends to a self-hosted Gotify server
// (https://gotify.net/docs/pushmsg) via that server's REST API and an
// application token (create one in Gotify's web UI under Apps -- NOT
// the same as a client/user token). BaseURL is the server root, e.g.
// "https://gotify.example.com" (no trailing slash needed, trimmed if
// present).
type Gotify struct {
	BaseURL string
	Token   string
}

func (g Gotify) Send(ctx context.Context, msg Message) error {
	base := strings.TrimRight(g.BaseURL, "/")
	endpoint := base + "/message?token=" + url.QueryEscape(g.Token)
	body := map[string]any{
		"message": truncate(msg.Text, MaxLengthGotify),
	}
	if msg.Title != "" {
		body["title"] = msg.Title
	} else {
		body["title"] = "cs-send" // Gotify requires a non-empty title
	}
	if msg.Priority > 0 {
		body["priority"] = msg.Priority
	}
	if err := postJSON(ctx, endpoint, body, nil); err != nil {
		return fmt.Errorf("gotify: %w", err)
	}
	return nil
}
