package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

// postJSON POSTs body (marshaled to JSON) to url with the given extra
// headers, and treats any non-2xx response as an error including the
// response body (every provider here returns a useful error body on
// failure -- Discord/Slack/Telegram/Gotify all do, worth surfacing
// rather than just the status code).
func postJSON(ctx context.Context, url string, body any, headers map[string]string) error {
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	return postRaw(ctx, url, b, "application/json", headers)
}

// postRaw POSTs raw bytes with an explicit content type (ntfy takes
// plain text in the body, not JSON).
func postRaw(ctx context.Context, url string, body []byte, contentType string, headers map[string]string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("http %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
