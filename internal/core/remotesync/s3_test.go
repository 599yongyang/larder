package remotesync

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestS3RequestsAreSignedAndRoundTrip(t *testing.T) {
	var stored []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=AKID/") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if r.Header.Get("x-amz-date") == "" || r.Header.Get("x-amz-content-sha256") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodHead:
			w.WriteHeader(http.StatusOK)
		case http.MethodPut:
			stored, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			_, _ = w.Write(stored)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()
	client, err := NewS3(server.URL, "us-east-1", "bucket", "AKID", "SECRET", "larder", "data.zip")
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
