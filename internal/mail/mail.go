// Package mail sends an email via SMTP over TLS (implicit TLS on port
// 465, or STARTTLS elsewhere), with an HTML body and both inline
// (Content-ID referenced, e.g. <img src="cid:logo">) and regular
// attachments. Pure standard library (net/smtp, crypto/tls, mime/
// multipart) -- no external dependency.
package mail

// Attachment is either an inline part (referenced from the HTML body
// via "cid:"+CID, e.g. an embedded chart image) or a regular attachment
// (shown by the mail client as a downloadable file, not inline in the
// body).
type Attachment struct {
	Filename    string
	Data        []byte
	ContentType string // guessed from Filename's extension if empty
	Inline      bool
	CID         string // required when Inline is true; NOT used otherwise
}

// Message is one email to send. From/To/Cc/Bcc are full RFC 5322
// address strings ("Name <addr@example.com>" or a bare address) --
// this package does no address validation or parsing of its own beyond
// what mime/multipart needs; a malformed address is rejected by the
// SMTP server itself (surfaced as a normal Send error).
type Message struct {
	From string
	To   []string
	Cc   []string
	Bcc  []string

	Subject string
	Text    string // plain-text alternative; optional but recommended for deliverability
	HTML    string // HTML body; optional -- Text-only if empty

	Attachments []Attachment
}
