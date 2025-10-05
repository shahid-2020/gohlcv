package httpclient

import (
	"context"
	"net/http"
)

type Doer interface {
	Do(ctx context.Context, req *http.Request) (*http.Response, error)
}
