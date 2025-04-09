package firefly

import (
	"context"
	"os"

	"github.com/go-json-experiment/json"
)

type Fetch struct {
	Path string `arg:""`
}

func (f Fetch) Run(ctx context.Context, a API) error {
	var resp any
	if err := Do(ctx, a, f.Path, nil, &resp, nil); err != nil {
		return err
	}
	return json.MarshalWrite(os.Stdout, resp)
}
