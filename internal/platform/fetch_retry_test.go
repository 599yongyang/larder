package platform

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPFetcherRetriesTruncatedBody(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 3 {
			w.Header().Set("Content-Length", "100")
			_, _ = w.Write([]byte("short"))
			return
		}
		_, _ = w.Write([]byte("complete"))
	}))
	defer server.Close()
	data, err := (HTTPFetcher{}).Fetch(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "complete" || attempts != 3 {
		t.Fatalf("data=%q attempts=%d", data, attempts)
	}
}
