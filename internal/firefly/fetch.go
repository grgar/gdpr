package firefly

import (
	"context"
	"net/url"
	"os"

	"github.com/go-json-experiment/json"
)

type Fetch struct {
	Path  string            `arg:""`
	Query map[string]string `short:"q"`
}

func (f Fetch) Run(ctx context.Context, a API) error {
	q := make(url.Values, len(f.Query))
	for k, v := range f.Query {
		q.Add(k, v)
	}
	var resp any
	if err := Do(ctx, a, f.Path, q, &resp, nil); err != nil {
		return err
	}
	return json.MarshalWrite(os.Stdout, resp)
}
