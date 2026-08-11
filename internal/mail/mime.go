package mail

import (
	"bytes"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/textproto"
	"path/filepath"
	"strings"
)

// build renders msg as a full RFC 5322 message (headers + MIME body),
// ready to hand straight to SMTP DATA. allTo is every envelope
// recipient (To+Cc+Bcc combined) -- used only by smtp.go for the SMTP
// RCPT TO commands, never written into the header block here (Bcc
// addresses must never appear in delivered headers, or every recipient
// would see the Bcc list -- this is the one place that distinction
// matters and is easy to get wrong).
func build(msg Message) ([]byte, error) {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "From: %s\r\n", msg.From)
	if len(msg.To) > 0 {
		fmt.Fprintf(&buf, "To: %s\r\n", strings.Join(msg.To, ", "))
	}
	if len(msg.Cc) > 0 {
		fmt.Fprintf(&buf, "Cc: %s\r\n", strings.Join(msg.Cc, ", "))
	}
	fmt.Fprintf(&buf, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", msg.Subject))
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")

	var inline, attachments []Attachment
	for _, a := range msg.Attachments {
		if a.Inline {
			inline = append(inline, a)
		} else {
			attachments = append(attachments, a)
		}
	}

	// bodyPart renders the Text/HTML alternative (or just one of them)
	// as its own self-contained MIME blob, WITHOUT top-level headers
	// (From/To/Subject) -- those are only written once, above, for the
	// whole message. Everything from here down builds nested multipart
	// structures as needed and writes exactly one final top-level
	// Content-Type header for the outermost part.
	bodyBuf, bodyContentType, err := renderAlternative(msg.Text, msg.HTML)
	if err != nil {
		return nil, err
	}

	switch {
	case len(inline) == 0 && len(attachments) == 0:
		// Simplest case: just the alternative part(s) as the whole
		// message body.
		buf.WriteString("Content-Type: " + bodyContentType + "\r\n\r\n")
		buf.Write(bodyBuf)

	case len(inline) > 0 && len(attachments) == 0:
		relBuf, relContentType, err := wrapRelated(bodyBuf, bodyContentType, inline)
		if err != nil {
			return nil, err
		}
		buf.WriteString("Content-Type: " + relContentType + "\r\n\r\n")
		buf.Write(relBuf)

	default:
		// Attachments present (with or without inline images): outer
		// multipart/mixed, first part is either the plain alternative
		// or a multipart/related (if there are also inline images),
		// followed by one part per real attachment.
		mw := multipart.NewWriter(&buf)
		buf.Reset() // multipart.NewWriter needs a writer; rebuild header block below with the real boundary
		var hdr bytes.Buffer
		fmt.Fprintf(&hdr, "From: %s\r\n", msg.From)
		if len(msg.To) > 0 {
			fmt.Fprintf(&hdr, "To: %s\r\n", strings.Join(msg.To, ", "))
		}
		if len(msg.Cc) > 0 {
			fmt.Fprintf(&hdr, "Cc: %s\r\n", strings.Join(msg.Cc, ", "))
		}
		fmt.Fprintf(&hdr, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", msg.Subject))
		fmt.Fprintf(&hdr, "MIME-Version: 1.0\r\n")
		fmt.Fprintf(&hdr, "Content-Type: multipart/mixed; boundary=%q\r\n\r\n", mw.Boundary())
		buf.Write(hdr.Bytes())

		firstBuf, firstContentType := bodyBuf, bodyContentType
		if len(inline) > 0 {
			firstBuf, firstContentType, err = wrapRelated(bodyBuf, bodyContentType, inline)
			if err != nil {
				return nil, err
			}
		}
		part, err := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {firstContentType}})
		if err != nil {
			return nil, err
		}
		part.Write(firstBuf)

		for _, a := range attachments {
			if err := writeAttachmentPart(mw, a, false); err != nil {
				return nil, err
			}
		}
		mw.Close()
	}

	return buf.Bytes(), nil
}

// renderAlternative builds either a single text/plain or text/html
// part, or (when both are given) a multipart/alternative containing
// both -- text/plain first, per RFC 2046's recommendation that clients
// prefer the LAST alternative they understand, so the richer HTML part
// goes last.
func renderAlternative(text, html string) (body []byte, contentType string, err error) {
	switch {
	case text != "" && html != "":
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		if err := writeTextPart(mw, "text/plain", text); err != nil {
			return nil, "", err
		}
		if err := writeTextPart(mw, "text/html", html); err != nil {
			return nil, "", err
		}
		mw.Close()
		return buf.Bytes(), fmt.Sprintf("multipart/alternative; boundary=%q", mw.Boundary()), nil
	case html != "":
		return encodeQP(html), "text/html; charset=\"utf-8\"", nil
	default:
		return encodeQP(text), "text/plain; charset=\"utf-8\"", nil
	}
}

func writeTextPart(mw *multipart.Writer, contentType, text string) error {
	part, err := mw.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {contentType + "; charset=\"utf-8\""},
		"Content-Transfer-Encoding": {"quoted-printable"},
	})
	if err != nil {
		return err
	}
	_, err = part.Write(encodeQP(text))
	return err
}

func encodeQP(s string) []byte {
	var buf bytes.Buffer
	w := quotedprintable.NewWriter(&buf)
	w.Write([]byte(s))
	w.Close()
	return buf.Bytes()
}

// wrapRelated wraps an already-rendered body part (bodyBuf/bodyContentType,
// e.g. the multipart/alternative from renderAlternative, or a plain
// text/html part) together with inline attachments into a
// multipart/related, per RFC 2387 -- what makes <img src="cid:XXX"> in
// the HTML resolve to the attached image data.
func wrapRelated(bodyBuf []byte, bodyContentType string, inline []Attachment) ([]byte, string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {bodyContentType}})
	if err != nil {
		return nil, "", err
	}
	part.Write(bodyBuf)
	for _, a := range inline {
		if a.CID == "" {
			return nil, "", fmt.Errorf("inline attachment %q: CID is required for inline attachments", a.Filename)
		}
		if err := writeAttachmentPart(mw, a, true); err != nil {
			return nil, "", err
		}
	}
	mw.Close()
	return buf.Bytes(), fmt.Sprintf("multipart/related; boundary=%q", mw.Boundary()), nil
}

func writeAttachmentPart(mw *multipart.Writer, a Attachment, inline bool) error {
	ct := a.ContentType
	if ct == "" {
		ct = mime.TypeByExtension(filepath.Ext(a.Filename))
		if ct == "" {
			ct = "application/octet-stream"
		}
	}
	h := textproto.MIMEHeader{
		"Content-Type":              {fmt.Sprintf("%s; name=%q", ct, a.Filename)},
		"Content-Transfer-Encoding": {"base64"},
	}
	if inline {
		h.Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", a.Filename))
		h.Set("Content-ID", "<"+a.CID+">")
	} else {
		h.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", a.Filename))
	}
	part, err := mw.CreatePart(h)
	if err != nil {
		return err
	}
	enc := base64WrapWriter{w: part}
	_, err = enc.Write(a.Data)
	return err
}
