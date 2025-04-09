package firefly

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/go-json-experiment/json"
)

type API struct {
	Endpoint *url.URL `short:"e" help:"URL to Firefly" required:"" env:"FIREFLY_URL,FIREFLY_III_URL"`
	Token    string   `short:"t" help:"Access token (generate at /profile)" required:"" xor:"token,token-file" env:"FIREFLY_ACCESS_TOKEN"`
}

func (a API) authHeader() http.Header {
	h := http.Header{}
	h.Add("Content-Type", "application/json")
	h.Add("Accept", "application/vnd.api+json")
	h.Add("Authorization", "Bearer "+a.Token)
	return h
}

func Get[T any](ctx context.Context, a API, path string, resp *T) error {
	u, err := url.JoinPath(a.Endpoint.String(), path)
	if err != nil {
		panic(err)
	}
	slog.Info("making request", slog.String("url", u))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header = a.authHeader()
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return json.UnmarshalRead(res.Body, &resp)
}
