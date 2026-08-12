package chat

import "context"

// Discord sends via a channel's Incoming Webhook URL
// (Server Settings > Integrations > Webhooks > New Webhook, then copy
// the URL) -- no bot, no OAuth, no app review.
type Discord struct {
	WebhookURL string
}

func (d Discord) Send(ctx context.Context, msg Message) error {
	text := msg.Text
	if msg.Title != "" {
		text = "**" + msg.Title + "**\n" + text
	}
	// Discord's webhook payload: https://discord.com/developers/docs/resources/webhook#execute-webhook
	// "content" is the only field this needs. Discord's own 2000-char
	// hard limit is enforced here (truncate, see truncate.go) BEFORE
	// sending -- so a caller handing this a long report gets a clearly
	// marked truncated alert instead of a raw 400 rejection.
	text = truncate(text, MaxLengthDiscord)
	return postJSON(ctx, d.WebhookURL, map[string]string{"content": text}, nil)
}
