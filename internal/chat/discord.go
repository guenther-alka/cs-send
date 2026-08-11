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
	// "content" is the only field this needs; Discord caps it at 2000
	// chars server-side and returns 400 if exceeded -- not pre-validated
	// here, the caller sees Discord's own error message via postJSON's
	// error-body surfacing.
	return postJSON(ctx, d.WebhookURL, map[string]string{"content": text}, nil)
}
