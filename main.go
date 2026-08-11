// cs-send -- send email (SMTP/TLS, HTML + attachments) and chat alerts
// (Discord/Telegram/Slack/ntfy/Gotify) from the command line.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/guenther-alka/cs-send/internal/chat"
	"github.com/guenther-alka/cs-send/internal/mail"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "version":
		fmt.Println("cs-send " + version)
	case "mail":
		mailCmd(os.Args[2:])
	case "chat":
		chatCmd(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`cs-send -- mail + chat alerts from the command line

Usage:
  cs-send mail --to <addr>[,<addr>...] --subject <s> [options]
  cs-send chat --provider discord|telegram|slack|ntfy|gotify [options]
  cs-send version

MAIL:
  --to, --cc, --bcc <addr>[,<addr>...]   recipients (--to required)
  --from <addr>                          default: --user
  --subject <s>
  --text <s> | --text-file <path>        plain-text body
  --html <s> | --html-file <path>        HTML body
  --attach <path>[,<path>...]            regular attachments
  --inline <path>=<cid>[,<path>=<cid>...]  inline images, referenced in
                                          --html as cid:<cid>, e.g.
                                          <img src="cid:logo">
  --smtp-host <host>                     required
  --smtp-port <port>                     default 587 (STARTTLS);
                                          465 = implicit TLS
  --user <addr>                          SMTP auth username
  --key <password>                       SMTP auth password (for Gmail:
                                          an App Password, not your
                                          normal account password)
  --insecure-skip-verify                 skip TLS cert verification
                                          (self-hosted relay with a
                                          self-signed cert only)

CHAT:
  --provider discord|telegram|slack|ntfy|gotify   required
  --text <s>                             required
  --title <s>                            optional (ntfy/Gotify only;
                                          prepended to text elsewhere)
  --priority <1-5>                       optional (ntfy/Gotify only)

  --url <url>          discord/slack: the webhook URL
                        ntfy: the topic URL, e.g. https://ntfy.sh/mytopic
                        gotify: the server base URL, e.g. https://gotify.example.com
  --token <token>       gotify: the application token (with --url)
                        telegram: the bot token (with --chat-id)
  --chat-id <id>        telegram only, with --token`)
}

func mailCmd(args []string) {
	fs := flag.NewFlagSet("mail", flag.ExitOnError)
	to := fs.String("to", "", "required: comma-separated recipient addresses")
	cc := fs.String("cc", "", "comma-separated")
	bcc := fs.String("bcc", "", "comma-separated")
	from := fs.String("from", "", "default: --user")
	subject := fs.String("subject", "", "")
	text := fs.String("text", "", "plain-text body")
	textFile := fs.String("text-file", "", "read plain-text body from file")
	html := fs.String("html", "", "HTML body")
	htmlFile := fs.String("html-file", "", "read HTML body from file")
	attach := fs.String("attach", "", "comma-separated file paths")
	inline := fs.String("inline", "", "comma-separated path=cid pairs")
	smtpHost := fs.String("smtp-host", "", "required")
	smtpPort := fs.Int("smtp-port", 587, "587=STARTTLS, 465=implicit TLS")
	user := fs.String("user", "", "SMTP auth username")
	key := fs.String("key", "", "SMTP auth password (Gmail: App Password)")
	insecure := fs.Bool("insecure-skip-verify", false, "skip TLS cert verification")
	fs.Parse(args)

	if *to == "" {
		fmt.Fprintln(os.Stderr, "error: --to is required")
		os.Exit(2)
	}
	if *smtpHost == "" {
		fmt.Fprintln(os.Stderr, "error: --smtp-host is required")
		os.Exit(2)
	}

	bodyText := *text
	if *textFile != "" {
		b, err := os.ReadFile(*textFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: --text-file: %v\n", err)
			os.Exit(1)
		}
		bodyText = string(b)
	}
	bodyHTML := *html
	if *htmlFile != "" {
		b, err := os.ReadFile(*htmlFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: --html-file: %v\n", err)
			os.Exit(1)
		}
		bodyHTML = string(b)
	}
	if bodyText == "" && bodyHTML == "" {
		fmt.Fprintln(os.Stderr, "error: at least one of --text/--text-file/--html/--html-file is required")
		os.Exit(2)
	}

	var attachments []mail.Attachment
	for _, p := range splitCSV(*attach) {
		data, err := os.ReadFile(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: --attach %s: %v\n", p, err)
			os.Exit(1)
		}
		attachments = append(attachments, mail.Attachment{Filename: filepath.Base(p), Data: data})
	}
	for _, pair := range splitCSV(*inline) {
		p, cid, ok := strings.Cut(pair, "=")
		if !ok || p == "" || cid == "" {
			fmt.Fprintf(os.Stderr, "error: --inline entry %q must be path=cid\n", pair)
			os.Exit(2)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: --inline %s: %v\n", p, err)
			os.Exit(1)
		}
		attachments = append(attachments, mail.Attachment{Filename: filepath.Base(p), Data: data, Inline: true, CID: cid})
	}

	msg := mail.Message{
		From:        firstNonEmpty(*from, *user),
		To:          splitCSV(*to),
		Cc:          splitCSV(*cc),
		Bcc:         splitCSV(*bcc),
		Subject:     *subject,
		Text:        bodyText,
		HTML:        bodyHTML,
		Attachments: attachments,
	}
	cfg := mail.SMTPConfig{
		Host: *smtpHost, Port: *smtpPort, User: *user, Password: *key,
		InsecureSkipVerify: *insecure,
	}
	if err := mail.Send(cfg, msg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("sent")
}

func chatCmd(args []string) {
	fs := flag.NewFlagSet("chat", flag.ExitOnError)
	provider := fs.String("provider", "", "required: discord|telegram|slack|ntfy|gotify")
	text := fs.String("text", "", "required")
	title := fs.String("title", "", "optional (ntfy/gotify only)")
	priority := fs.Int("priority", 0, "optional 1-5 (ntfy/gotify only)")
	url := fs.String("url", "", "webhook/topic/base URL, see usage per provider")
	token := fs.String("token", "", "gotify app token, or telegram bot token")
	chatID := fs.String("chat-id", "", "telegram only")
	fs.Parse(args)

	if *text == "" {
		fmt.Fprintln(os.Stderr, "error: --text is required")
		os.Exit(2)
	}

	var n chat.Notifier
	switch *provider {
	case "discord":
		if *url == "" {
			fmt.Fprintln(os.Stderr, "error: discord requires --url (the webhook URL)")
			os.Exit(2)
		}
		n = chat.Discord{WebhookURL: *url}
	case "slack":
		if *url == "" {
			fmt.Fprintln(os.Stderr, "error: slack requires --url (the webhook URL)")
			os.Exit(2)
		}
		n = chat.Slack{WebhookURL: *url}
	case "ntfy":
		if *url == "" {
			fmt.Fprintln(os.Stderr, "error: ntfy requires --url (the topic URL)")
			os.Exit(2)
		}
		n = chat.Ntfy{URL: *url}
	case "gotify":
		if *url == "" || *token == "" {
			fmt.Fprintln(os.Stderr, "error: gotify requires --url (server base URL) and --token (app token)")
			os.Exit(2)
		}
		n = chat.Gotify{BaseURL: *url, Token: *token}
	case "telegram":
		if *token == "" || *chatID == "" {
			fmt.Fprintln(os.Stderr, "error: telegram requires --token (bot token) and --chat-id")
			os.Exit(2)
		}
		n = chat.Telegram{BotToken: *token, ChatID: *chatID}
	default:
		fmt.Fprintln(os.Stderr, "error: --provider must be one of discord, telegram, slack, ntfy, gotify")
		os.Exit(2)
	}

	err := n.Send(context.Background(), chat.Message{Text: *text, Title: *title, Priority: *priority})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("sent")
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
