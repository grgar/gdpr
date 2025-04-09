package firefly

import (
	"context"
	"log/slog"
)

type Version struct{}

func (Version) Run(ctx context.Context, a API) error {
	var resp struct {
		Data map[string]string `json:"data"`
	}
	if err := Get(ctx, a, "api/v1/about", &resp); err != nil {
		return err
	}
	slog.Info("about", slog.String("version", resp.Data["version"]))
	return nil
}
