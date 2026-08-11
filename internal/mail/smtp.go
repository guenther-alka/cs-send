package mail

import (
	"crypto/tls"
	"fmt"
	"net"
	stdmail "net/mail"
	"net/smtp"
	"strconv"
)

// SMTPConfig holds connection + auth details for one SMTP server.
// TLS mode is determined by Port, not a separate flag: 465 means
// implicit TLS (the connection is TLS from the first byte -- Gmail,
// most providers' "SSL" port); any other port attempts STARTTLS after
// a plaintext connect (587 is the near-universal STARTTLS submission
// port today; 25 also works if the server offers STARTTLS, though most
// providers block unauthenticated/relay use on 25 entirely).
type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string

	// InsecureSkipVerify disables TLS certificate verification.
	// Deliberately NOT wired to a short/easy CLI flag name -- see
	// main.go's --insecure-skip-verify, which requires spelling it out
	// in full. This is for a self-hosted mail relay with a self-signed
	// cert, not a shortcut for real providers.
	InsecureSkipVerify bool
}

// Send builds msg per RFC 5322/2045/2387 (see mime.go) and delivers it
// over SMTP to every recipient in msg.To+Cc+Bcc. Bcc addresses are
// given to the SMTP envelope (RCPT TO) exactly like To/Cc, but -- as
// they must be -- never appear in the delivered message's own headers
// (see build's doc comment).
func Send(cfg SMTPConfig, msg Message) error {
	if cfg.Host == "" {
		return fmt.Errorf("smtp: host is required")
	}
	if len(msg.To) == 0 && len(msg.Cc) == 0 && len(msg.Bcc) == 0 {
		return fmt.Errorf("smtp: at least one recipient (To/Cc/Bcc) is required")
	}

	raw, err := build(msg)
	if err != nil {
		return fmt.Errorf("build message: %w", err)
	}

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	tlsConf := &tls.Config{ServerName: cfg.Host, InsecureSkipVerify: cfg.InsecureSkipVerify}

	var client *smtp.Client
	if cfg.Port == 465 {
		conn, err := tls.Dial("tcp", addr, tlsConf)
		if err != nil {
			return fmt.Errorf("tls dial %s: %w", addr, err)
		}
		client, err = smtp.NewClient(conn, cfg.Host)
		if err != nil {
			return fmt.Errorf("smtp handshake: %w", err)
		}
	} else {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			return fmt.Errorf("dial %s: %w", addr, err)
		}
		client, err = smtp.NewClient(conn, cfg.Host)
		if err != nil {
			return fmt.Errorf("smtp handshake: %w", err)
		}
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(tlsConf); err != nil {
				return fmt.Errorf("starttls: %w", err)
			}
		}
		// A server that offers neither implicit TLS (port 465) nor
		// STARTTLS is sent to in plaintext here -- deliberately not
		// treated as an error: some purely-internal relays (e.g. a
		// local Postfix with no TLS configured at all) are a legitimate,
		// if less common, target. cs-send does not silently downgrade a
		// connection that WAS offered STARTTLS -- only a server that
		// never offered it at all falls through to plaintext.
	}
	defer client.Close()

	if cfg.User != "" {
		auth := smtp.PlainAuth("", cfg.User, cfg.Password, cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}

	from := cfg.User
	if from == "" {
		from = msg.From
	}
	if err := client.Mail(bareAddr(from)); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}
	for _, rcpt := range allRecipients(msg) {
		if err := client.Rcpt(bareAddr(rcpt)); err != nil {
			return fmt.Errorf("RCPT TO %s: %w", rcpt, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	if _, err := w.Write(raw); err != nil {
		return fmt.Errorf("write message body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("finish message body: %w", err)
	}
	return client.Quit()
}

func allRecipients(msg Message) []string {
	var all []string
	all = append(all, msg.To...)
	all = append(all, msg.Cc...)
	all = append(all, msg.Bcc...)
	return all
}

// bareAddr extracts just the address part from an RFC 5322 address
// string ("Name <addr@example.com>" -> "addr@example.com") -- SMTP's
// MAIL FROM/RCPT TO commands take a bare address, not the display-name
// form that's valid (and common) in From/To header values and on this
// tool's own CLI. Falls back to the input unchanged if it doesn't
// parse as an RFC 5322 address (so a plain "addr@example.com" with no
// angle brackets, the common case, passes through untouched).
func bareAddr(s string) string {
	if a, err := stdmail.ParseAddress(s); err == nil {
		return a.Address
	}
	return s
}
