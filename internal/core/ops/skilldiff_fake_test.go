package ops_test

import "context"

type fakeFetcher struct{ data []byte }

func (f fakeFetcher) Fetch(context.Context, string) ([]byte, error) { return f.data, nil }
