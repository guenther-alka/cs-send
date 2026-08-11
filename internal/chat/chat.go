// Package chat sends a short alert message to a chat/notification
// provider (Discord, Telegram, Slack, ntfy, Gotify) via a single
// outbound HTTPS request -- no OAuth flow, no app review, just a
// webhook URL or a bot token the operator already has.
package chat

import "context"

// Message is a provider-agnostic notification. Not every field is used
// by every provider: Title is rendered by ntfy (a header) and Gotify (a
// JSON field); Discord, Slack, and Telegram have no separate title
// concept, so a non-empty Title is simply prepended to Text as a bold
// line for those three. Priority is only meaningful to ntfy (1-5,
// default 3) and Gotify (0-10ish, default 5) -- providers that ignore
// it just ignore it, not an error.
type Message struct {
	Text     string
	Title    string
	Priority int // 0 = provider default
}

// Notifier sends one Message to one configured destination.
type Notifier interface {
	Send(ctx context.Context, msg Message) error
}
