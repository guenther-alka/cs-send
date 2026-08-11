# cs-send

Send email (SMTP over TLS, HTML body with inline images and regular
attachments) and chat alerts (Discord, Telegram, Slack, ntfy, Gotify)
from the command line. One static binary, pure Go standard library --
no external dependencies at all.

Part of the [napp-it CS](https://napp-it.org) toolset (same build model
as [cs-sync](https://github.com/guenther-alka/cs-sync) and
[cs-stream](https://github.com/guenther-alka/cs-stream)), but fully
usable standalone.

---

## Mail

```
cs-send mail \
  --to alice@example.com,bob@example.com \
  --subject "Backup finished" \
  --html-file report.html \
  --attach chart.png \
  --smtp-host smtp.gmail.com --smtp-port 587 \
  --user you@gmail.com --key <app-password>
```

**TLS mode follows the port**, not a separate flag: `465` = implicit
TLS from the first byte (Gmail's/most providers' "SSL" port); anything
else attempts STARTTLS after a plaintext connect (`587` is the
near-universal STARTTLS submission port today).

**Gmail note:** "less secure app access" no longer exists. You need a
Google Account with 2-Step Verification enabled, then an
[App Password](https://myaccount.google.com/apppasswords) generated
for it -- use that as `--key`, not your normal account password.
Outlook/Office365 and generic SMTP relays work the same way, with
whatever password/app-password scheme that provider requires.

**Inline images** (embedded in the HTML body, e.g. `<img src="cid:logo">`
rather than a downloadable attachment):

```
cs-send mail --to a@b.com --subject "Status" \
  --html '<html><body><h1>All good</h1><img src="cid:chart"></body></html>' \
  --inline chart.png=chart \
  --smtp-host smtp.example.com --smtp-port 587 --user u --key p
```

`--inline path=cid` can be repeated (comma-separated:
`--inline a.png=img1,b.png=img2`); `--attach path` for regular
(non-inline, downloadable) attachments works the same way.

Full flag reference: `cs-send mail` with no other arguments prints
usage.

**`--logfile <path>`** (both `mail` and `chat`) appends one timestamped
result line per run -- useful when called from cron or a napp-it CS
job, where stdout/stderr don't persist. Never contains credentials:
`--key`/SMTP passwords, chat webhook URLs, and bot tokens are all
redacted from what gets written (a webhook URL grants send access to
anyone who has it, same threat model as a password).

---

## Chat

One outbound HTTPS request per provider -- no OAuth flow, no app
review, just a webhook URL or a bot token you already have.

```
cs-send chat --provider discord  --url  <webhook-url> --text "Deploy finished"
cs-send chat --provider slack    --url  <webhook-url> --text "Deploy finished"
cs-send chat --provider ntfy     --url  https://ntfy.sh/my-topic --text "Disk 90% full" --priority 5
cs-send chat --provider gotify   --url  https://gotify.example.com --token <app-token> --text "Scrub done"
cs-send chat --provider telegram --token <bot-token> --chat-id <id> --text "Backup failed"
```

| Provider | What you need | Setup |
|---|---|---|
| **Discord** | a webhook URL | Server Settings → Integrations → Webhooks → New Webhook → copy URL |
| **Slack** | a webhook URL | classic Incoming Webhook (still works for existing workspaces) |
| **ntfy** | a topic URL | pick any topic name on ntfy.sh, or self-host; no account needed for a public topic |
| **Gotify** | a server URL + app token | self-hosted; create an app in Gotify's web UI to get the token |
| **Telegram** | a bot token + chat ID | message @BotFather → `/newbot` → copy token; message @userinfobot to find your chat ID |

`--title` and `--priority` are honored by ntfy and Gotify (both have a
native concept of each); the other three providers have no separate
title field, so a `--title` is just prepended to `--text` as a bold
line instead.

Not included (deliberately, for now): WhatsApp (Business API is
heavily gated behind Meta verification -- disproportionate for an
alert tool), Signal (no official webhook API, would need a separate
`signal-cli` daemon), Microsoft Teams (classic incoming webhooks are
being retired in favor of Power Automate workflows -- meaningfully
more setup than the others here). Matrix was considered and left out
of this first pass too -- doable (an access token + a REST PUT, no
OAuth) but a notch more setup than the five above.

---

## Install

Prebuilt binaries for every platform are attached to each
[GitHub release](https://github.com/guenther-alka/cs-send/releases)
(`cs-send-<os>.<arch>.tar.gz`: `mswin.amd64`, `linux.amd64`,
`linux.arm64`, `illumos.amd64`, `solaris.amd64`, `freebsd.amd64`,
`darwin.amd64`, `darwin.arm64`). Extract and run -- no install step,
no runtime dependency beyond the OS's own TLS/CA trust store.

## Build from source

```
git clone https://github.com/guenther-alka/cs-send.git
cd cs-send
go build -o cs-send .
```

Cross-compile with `GOOS`/`GOARCH`:

```
GOOS=linux GOARCH=amd64 go build -o cs-send .
```

## Tests

```
go build ./...
go vet ./...
go test ./...
```

Unit tests cover: the MIME message builder (`internal/mail`) --
plain-text-only, text+HTML alternative ordering, inline-image
`Content-ID` wiring, regular attachments, and the Bcc-never-in-headers
invariant, all verified by parsing the built message back with
`net/mail`/`mime/multipart` rather than just string-matching; and every
chat provider (`internal/chat`) against a local `httptest.Server`,
including error-body surfacing on a non-2xx response. `mail.Send`
itself (the live SMTP conversation) isn't covered by an automated test
-- that needs a real server -- and was exercised manually against Gmail
and a local relay during development.

---

## License

BSD 2-Clause -- see [LICENSE](LICENSE).

## Warranty

None. Verify recipients and attachments before sending anything that
matters; treat SMTP credentials and chat tokens like any other secret
(don't commit them, don't pass them on a shared machine's command line
history if that's a concern -- environment variables or a wrapper
script are safer than a bare CLI flag for anything long-lived).
