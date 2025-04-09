package firefly

import (
	"context"
	"log/slog"
)

type Version struct{}

func (Version) Run(ctx context.Context, a API) error {
	var resp struct {
		Version string `json:"version"`
	}
	if err := Do(ctx, a, "about", nil, &resp, nil); err != nil {
		return err
	}
	slog.Info("about", slog.String("version", resp.Version))
	return nil
}
