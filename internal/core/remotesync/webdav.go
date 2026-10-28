package remotesync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxSyncPackageBytes = 64 << 20

type WebDAV struct {
	BaseURL  string
	Username string
	Password string
	Filename string
	Client   *http.Client
}

func NewWebDAV(baseURL, username, password, filename string) (*WebDAV, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil, errors.New("WebDAV URL is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("WebDAV URL is invalid")
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname())) {
		return nil, errors.New("WebDAV URL must use HTTPS (HTTP is only allowed for localhost)")
	}
	filename = strings.TrimSpace(filename)
	if filename == "" {
		filename = "larder-data.zip"
	}
	if strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return nil, errors.New("WebDAV file name must not contain a path")
	}
	return &WebDAV{BaseURL: strings.TrimRight(baseURL, "/"), Username: username, Password: password, Filename: filename, Client: &http.Client{Timeout: 30 * time.Second}}, nil
}

func (w *WebDAV) fileURL() string {
	return w.BaseURL + "/" + url.PathEscape(w.Filename)
}

func (w *WebDAV) request(ctx context.Context, method, target string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, err
	}
	if w.Username != "" || w.Password != "" {
		req.SetBasicAuth(w.Username, w.Password)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if method == "PROPFIND" {
		req.Header.Set("Depth", "0")
	}
	resp, err := w.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4*1024))
		return nil, fmt.Errorf("WebDAV %s: HTTP %d", method, resp.StatusCode)
	}
	return resp, nil
}

func (w *WebDAV) Test(ctx context.Context) error {
	body := `<?xml version="1.0" encoding="utf-8"?><propfind xmlns="DAV:"><prop><allprop/></prop></propfind>`
	resp, err := w.request(ctx, "PROPFIND", w.BaseURL, strings.NewReader(body), "application/xml")
	if err != nil {
		// Some servers do not implement PROPFIND at the collection root;
		// fall back to checking whether the target endpoint is reachable.
		resp, err = w.request(ctx, "HEAD", w.BaseURL, nil, "")
		if err != nil {
			return err
		}
	}
	resp.Body.Close()
	return nil
}

func (w *WebDAV) Upload(ctx context.Context, data []byte) error {
	if len(data) > maxSyncPackageBytes {
		return errors.New("sync package is too large")
	}
	resp, err := w.request(ctx, "PUT", w.fileURL(), bytes.NewReader(data), "application/zip")
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (w *WebDAV) Download(ctx context.Context) ([]byte, error) {
	resp, err := w.request(ctx, "GET", w.fileURL(), nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxSyncPackageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxSyncPackageBytes {
		return nil, errors.New("sync package is too large")
	}
	return raw, nil
}

func isLoopbackHost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || strings.HasSuffix(host, ".localhost")
}
