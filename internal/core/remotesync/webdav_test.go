package remotesync

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebDAVUploadDownloadAndAuth(t *testing.T) {
	var stored []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "alice" || pass != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.Method {
		case "PROPFIND", "HEAD":
			w.WriteHeader(http.StatusMultiStatus)
		case "PUT":
			raw, _ := io.ReadAll(r.Body)
			stored = raw
			w.WriteHeader(http.StatusCreated)
		case "GET":
			if stored == nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write(stored)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()
	client, err := NewWebDAV(server.URL, "alice", "secret", "larder-data.zip")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := client.Test(ctx); err != nil {
		t.Fatal(err)
	}
	if err := client.Upload(ctx, []byte("package")); err != nil {
		t.Fatal(err)
	}
	raw, err := client.Download(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "package" {
		t.Fatalf("download = %q", raw)
	}
}

func TestWebDAVRejectsUnsafeFilename(t *testing.T) {
	if _, err := NewWebDAV("https://example.com/dav", "", "", "../secret.zip"); err == nil {
		t.Fatal("expected path rejection")
	}
}
