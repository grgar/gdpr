package firefly

import (
	"context"
	"net/url"
	"os"

	"github.com/go-json-experiment/json"
)

type Fetch struct {
	Path   string            `arg:""`
	Query  map[string]string `short:"q"`
	Method string            `short:"X" default:"GET" help:"HTTP method to use"`
}

func (f Fetch) Run(ctx context.Context, a API) error {
	q := make(url.Values, len(f.Query))
	for k, v := range f.Query {
		q.Add(k, v)
	}
	var resp any
	if err := Do(ctx, a, f.Method, f.Path, q, &resp, nil); err != nil {
		return err
	}
	if resp == nil {
		return nil
	}
	return json.MarshalWrite(os.Stdout, resp)
}
