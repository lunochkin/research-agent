package llm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

const maxAttempts = 6

// postJSON POSTs body and returns the response bytes, retrying on 429 / 5xx with
// backoff (honoring Retry-After). Rebuilds the request each attempt since the
// body reader is consumed. Respects ctx cancellation during waits.
func postJSON(ctx context.Context, client *http.Client, url string, headers map[string]string, body []byte) ([]byte, error) {
	var lastErr error
	for attempt := range maxAttempts {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if !sleepBackoff(ctx, attempt, 0) {
				return nil, err
			}
			continue
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		switch {
		case resp.StatusCode == http.StatusOK:
			return raw, nil
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			lastErr = fmt.Errorf("%s: %s", resp.Status, raw)
			if !sleepBackoff(ctx, attempt, retryAfter(resp.Header)) {
				return nil, lastErr
			}
		default:
			return nil, fmt.Errorf("%s: %s", resp.Status, raw)
		}
	}
	return nil, fmt.Errorf("exhausted retries: %w", lastErr)
}

// retryAfter reads the Retry-After header (seconds) if present, else 0.
func retryAfter(h http.Header) time.Duration {
	if v := h.Get("Retry-After"); v != "" {
		if secs, err := strconv.ParseFloat(v, 64); err == nil {
			return time.Duration(secs * float64(time.Second))
		}
	}
	return 0
}

// sleepBackoff waits before the next attempt: the server hint if given, else
// exponential backoff with jitter. Returns false if ctx is cancelled.
func sleepBackoff(ctx context.Context, attempt int, hint time.Duration) bool {
	d := hint
	if d <= 0 {
		d = time.Duration(1<<attempt)*time.Second + time.Duration(rand.Intn(500))*time.Millisecond
	}
	if d > 30*time.Second {
		d = 30 * time.Second
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}
