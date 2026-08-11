package mail

import (
	"encoding/base64"
	"io"
)

// base64WrapWriter base64-encodes everything written to it and inserts
// a CRLF every 76 encoded characters, per RFC 2045's line-length limit
// for base64-encoded MIME body parts (most mail clients/servers accept
// unwrapped base64 in practice, but staying within spec avoids relying
// on that leniency).
type base64WrapWriter struct {
	w       io.Writer
	lineLen int
}

const base64LineLength = 76

func (b *base64WrapWriter) Write(p []byte) (int, error) {
	enc := base64.StdEncoding.EncodeToString(p)
	written := 0
	for len(enc) > 0 {
		remaining := base64LineLength - b.lineLen
		chunk := enc
		if len(chunk) > remaining {
			chunk = chunk[:remaining]
		}
		n, err := b.w.Write([]byte(chunk))
		written += n
		if err != nil {
			return written, err
		}
		b.lineLen += len(chunk)
		enc = enc[len(chunk):]
		if b.lineLen >= base64LineLength {
			if _, err := b.w.Write([]byte("\r\n")); err != nil {
				return written, err
			}
			b.lineLen = 0
		}
	}
	return len(p), nil
}
