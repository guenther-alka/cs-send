package chat

import (
	"context"
	"strconv"
)

// Ntfy sends to a ntfy.sh topic (or a self-hosted ntfy instance) --
// URL is the topic's own URL, e.g. "https://ntfy.sh/my-alerts" or
// "https://ntfy.example.com/my-alerts". No account/token needed for a
// public ntfy.sh topic (anyone who knows the topic name can read it --
// pick an unguessable name, or self-host with auth for anything
// sensitive); a self-hosted instance with access control configured
// would need an Authorization header, not implemented here (add a
// Token field if/when needed).
//
// Unlike every other provider here, ntfy's body IS the message text --
// no JSON envelope -- with metadata (title, priority) passed as HTTP
// headers instead. https://docs.ntfy.sh/publish/
type Ntfy struct {
	URL string
}

func (n Ntfy) Send(ctx context.Context, msg Message) error {
	headers := map[string]string{}
	if msg.Title != "" {
		headers["Title"] = msg.Title
	}
	if msg.Priority > 0 {
		headers["Priority"] = strconv.Itoa(msg.Priority) // 1 (min) .. 5 (max), ntfy default 3
	}
	text := truncate(msg.Text, MaxLengthNtfy)
	return postRaw(ctx, n.URL, []byte(text), "text/plain; charset=utf-8", headers)
}
