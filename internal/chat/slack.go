package chat

import "context"

// Slack sends via a classic Incoming Webhook URL (still supported for
// existing workspaces, though Slack has been steering new integrations
// toward Workflow Builder / the Slack app model -- a plain webhook URL
// remains the simplest path where it's still available).
type Slack struct {
	WebhookURL string
}

func (s Slack) Send(ctx context.Context, msg Message) error {
	text := msg.Text
	if msg.Title != "" {
		text = "*" + msg.Title + "*\n" + text
	}
	text = truncate(text, MaxLengthSlack)
	// https://api.slack.com/messaging/webhooks -- "text" is the only
	// required field for a plain message.
	return postJSON(ctx, s.WebhookURL, map[string]string{"text": text}, nil)
}
