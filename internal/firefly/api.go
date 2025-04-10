package firefly

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

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

type overrideReaderContextKey struct{}

var OverrideReaderContextKey = overrideReaderContextKey{}

func Do[T any](ctx context.Context, a API, method, path string, q url.Values, out *T, r io.Reader) error {
	u, err := url.JoinPath(a.Endpoint.String(), "api/v1", path)
	if err != nil {
		panic(err)
	}
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	slog.Info("making request", slog.String("url", u))
	var body io.Reader
	if value := ctx.Value(OverrideReaderContextKey); value != nil {
		body = value.(io.Reader)
	} else {
		req, err := http.NewRequestWithContext(ctx, method, u, r)
		if err != nil {
			return err
		}
		req.Header = a.authHeader()
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<10))
			return fmt.Errorf("status %d: %s", res.StatusCode, string(body))
		}
		body = res.Body
	}
	resp := struct {
		Data *T `json:"data"`
		Meta struct {
			Pagination struct {
				Count int `json:"count"`
				Total int `json:"total"`
			} `json:"pagination"`
		} `json:"meta"`
	}{Data: out}
	defer func() {
		slog.Info("pagination", slog.Int("count", resp.Meta.Pagination.Count), slog.Int("total", resp.Meta.Pagination.Total))
	}()
	return json.UnmarshalRead(body, &resp)
}

type StringInt int

func (s StringInt) MarshalText() ([]byte, error) {
	return []byte(strconv.Itoa(int(s))), nil
}

func (s *StringInt) UnmarshalText(b []byte) error {
	value, err := strconv.Atoi(string(b))
	if err != nil {
		return err
	}
	*s = StringInt(value)
	return nil
}

type StringFloat float64

func (s StringFloat) MarshalText() ([]byte, error) {
	return []byte(strconv.FormatFloat(float64(s), 'f', 2, 64)), nil
}

func (s *StringFloat) UnmarshalText(b []byte) error {
	value, err := strconv.ParseFloat(string(b), 64)
	if err != nil {
		return err
	}
	*s = StringFloat(value)
	return nil
}
