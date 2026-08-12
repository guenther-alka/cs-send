package chat

import "fmt"

// Per-provider maximum message length, applied automatically by every
// provider's Send() before it builds the outgoing request -- chat
// alerts must stay short (unlike mail, which has no comparable limit
// here); this is a safety net so a caller that accidentally hands a
// long report/log excerpt to chat.Send doesn't just get a raw 400 back
// from the provider.
//
// Discord (2000) and Telegram (4096) are the platform's own documented
// HARD limits -- exceeding them is rejected outright, not a style
// choice. Slack, ntfy, and Gotify don't enforce a comparably strict
// hard cap in practice, but a chat alert that's actually several
// thousand characters long stops being useful as an AT-A-GLANCE alert
// (the entire point of the chat channel vs. mail) -- their limits here
// are a deliberate practical ceiling, not a platform requirement.
const (
	MaxLengthDiscord  = 2000
	MaxLengthSlack    = 3000
	MaxLengthNtfy     = 1000
	MaxLengthGotify   = 2000
	MaxLengthTelegram = 4096
)

// truncate shortens s to at most max characters (bytes, to keep this
// simple and match the platforms' own byte-oriented limits -- not
// unicode-rune-aware, so a truncation could in principle split a
// multi-byte character; acceptable for an alert-truncation safety net,
// not attempted for anything that needs to round-trip exactly). Below
// the limit, s is returned completely unchanged. Above it, a trailing
// marker names how many characters were cut so the reader knows the
// alert is incomplete and where to look for the rest (mail, or
// whatever produced the original report) -- silently cutting off
// mid-sentence with no indication would be worse than the raw error
// this truncation exists to avoid.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	marker := fmt.Sprintf("\n... [truncated, %d chars total]", len(s))
	keep := max - len(marker)
	if keep < 0 {
		keep = 0
	}
	if keep > len(s) {
		keep = len(s)
	}
	return s[:keep] + marker
}
