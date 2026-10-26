package platform

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Fetcher is the read-only network surface used by explicit update checks.
type Fetcher interface {
	Fetch(ctx context.Context, url string) ([]byte, error)
}

type HTTPFetcher struct {
	Client *http.Client
}

func (f HTTPFetcher) Fetch(ctx context.Context, url string) ([]byte, error) {
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "larder/0.0.5 update-check")
		resp, err := client.Do(req)
		if err != nil {
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
			lastErr = err
		} else {
			if resp.StatusCode >= 500 {
				lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
				_ = resp.Body.Close()
			} else if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				_ = resp.Body.Close()
				return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
			} else {
				data, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
				_ = resp.Body.Close()
				if readErr == nil {
					return data, nil
				}
				lastErr = readErr
			}
		}
		if attempt < 2 {
			timer := time.NewTimer(time.Duration(attempt+1) * 400 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("download failed")
	}
	return nil, lastErr
}
