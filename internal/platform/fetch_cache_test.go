package platform

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type countingFetcher struct {
	calls atomic.Int32
}

func (f *countingFetcher) Fetch(context.Context, string) ([]byte, error) {
	f.calls.Add(1)
	return []byte("payload"), nil
}

func TestCachedFetcherReusesPayload(t *testing.T) {
	inner := &countingFetcher{}
	cached := NewCachedFetcher(inner, time.Minute, 2)
	for index := 0; index < 3; index++ {
		if _, err := cached.Fetch(context.Background(), "https://example.test/archive"); err != nil {
			t.Fatal(err)
		}
	}
	if inner.calls.Load() != 1 {
		t.Fatalf("fetch calls = %d, want 1", inner.calls.Load())
	}
}
