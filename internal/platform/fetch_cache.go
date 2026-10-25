package platform

import (
	"context"
	"sync"
	"time"
)

type fetchCacheEntry struct {
	data    []byte
	fetched time.Time
}

// CachedFetcher keeps a small, short-lived in-memory cache of upstream
// downloads. Update previews for multiple Skills in the same repository would
// otherwise download and extract the same tarball repeatedly.
type CachedFetcher struct {
	inner      Fetcher
	ttl        time.Duration
	maxEntries int
	mu         sync.Mutex
	entries    map[string]fetchCacheEntry
}

func NewCachedFetcher(inner Fetcher, ttl time.Duration, maxEntries int) *CachedFetcher {
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	if maxEntries <= 0 {
		maxEntries = 6
	}
	return &CachedFetcher{inner: inner, ttl: ttl, maxEntries: maxEntries, entries: map[string]fetchCacheEntry{}}
}

func (f *CachedFetcher) Fetch(ctx context.Context, url string) ([]byte, error) {
	f.mu.Lock()
	if entry, ok := f.entries[url]; ok && time.Since(entry.fetched) < f.ttl {
		data := entry.data
		f.mu.Unlock()
		return data, nil
	}
	f.mu.Unlock()

	data, err := f.inner.Fetch(ctx, url)
	if err != nil {
		return nil, err
	}

	f.mu.Lock()
	now := time.Now()
	for key, entry := range f.entries {
		if now.Sub(entry.fetched) >= f.ttl {
			delete(f.entries, key)
		}
	}
	for len(f.entries) >= f.maxEntries {
		var oldestKey string
		var oldest time.Time
		for key, entry := range f.entries {
			if oldestKey == "" || entry.fetched.Before(oldest) {
				oldestKey, oldest = key, entry.fetched
			}
		}
		delete(f.entries, oldestKey)
	}
	f.entries[url] = fetchCacheEntry{data: data, fetched: now}
	f.mu.Unlock()
	return data, nil
}
