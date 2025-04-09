package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"

	"github.com/alecthomas/kong"
)

type api struct {
	Endpoint *url.URL `short:"e" help:"URL to Firefly" required:"" env:"FIREFLY_URL,FIREFLY_III_URL"`
	Token    string   `short:"t" help:"Access token (generate at /profile)" required:"" xor:"token,token-file" env:"FIREFLY_ACCESS_TOKEN"`
}

func (a api) authHeader() http.Header {
	h := http.Header{}
	h.Add("Content-Type", "application/json")
	h.Add("Accept", "application/vnd.api+json")
	h.Add("Authorization", "Bearer "+a.Token)
	return h
}

func main() {
	var cli struct {
		API       api    `embed:""`
		TokenFile []byte `help:"Access token file path (instead of --token)" type:"filecontent"`

		Version struct{} `cmd:""`
	}
	ctx := kong.Parse(&cli)

	if len(cli.TokenFile) != 0 {
		cli.API.Token = strings.TrimSpace(string(cli.TokenFile))
	}

	var err error
	switch ctx.Command() {
	case "version":
		err = version(cli.API)
	default:
		panic(ctx.Command())
	}
	if err != nil {
		panic(err)
	}
}

func version(a api) error {
	u, err := url.JoinPath(a.Endpoint.String(), "api/v1/about")
	if err != nil {
		panic(err)
	}
	slog.Info("making request", slog.String("url", u))
	req, err := http.NewRequestWithContext(context.TODO(), http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header = a.authHeader()
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var resp struct {
		Data map[string]string `json:"data"`
	}
	err = json.UnmarshalRead(res.Body, &resp)
	if err != nil {
		return err
	}
	out, err := json.Marshal(&resp)
	if err != nil {
		return err
	}
	if err := (*jsontext.Value)(&out).Indent(); err != nil {
		return err
	}
	slog.Info("about", slog.String("version", resp.Data["version"]))
	return nil
}
