package chat

import "context"

// Telegram sends via a bot's sendMessage API
// (https://core.telegram.org/bots/api#sendmessage). Setup: message
// @BotFather in Telegram, /newbot, copy the token it gives you. ChatID
// is either your own numeric user ID (message @userinfobot to get it)
// or a group/channel ID the bot has been added to -- Telegram, not
// this package, is the source of truth for what IDs are valid.
//
// The one provider here that needs two pieces of config instead of a
// single webhook URL, because a bot token alone identifies the BOT, not
// a destination -- Telegram bots can message many chats, so which one
// is a separate, required parameter.
type Telegram struct {
	BotToken string
	ChatID   string

	// BaseURL overrides the default "https://api.telegram.org" --
	// empty means use the real API. Exists so tests can point Send at
	// an httptest.Server instead; not exposed on the CLI (there's no
	// legitimate operator reason to point at a different Telegram API
	// host).
	BaseURL string
}

func (t Telegram) Send(ctx context.Context, msg Message) error {
	text := msg.Text
	if msg.Title != "" {
		text = "*" + msg.Title + "*\n" + text
	}
	base := t.BaseURL
	if base == "" {
		base = "https://api.telegram.org"
	}
	endpoint := base + "/bot" + t.BotToken + "/sendMessage"
	body := map[string]string{
		"chat_id":    t.ChatID,
		"text":       text,
		"parse_mode": "Markdown",
	}
	return postJSON(ctx, endpoint, body, nil)
}
