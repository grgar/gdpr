package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"go.grg.app/gdpr/internal/firefly"

	"github.com/alecthomas/kong"
)

type CLI struct {
	API       firefly.API `embed:""`
	TokenFile []byte      `help:"Access token file path (instead of --token)" type:"filecontent"`

	Fetch   firefly.Fetch   `cmd:"" help:"Fetch from the given path"`
	Version firefly.Version `cmd:"" help:"Show version"`
	Link    firefly.Link    `cmd:"" help:"Link transactions to another"`
	Match   firefly.Match   `cmd:""`
}

func main() {
	var cli CLI
	k := kong.Parse(&cli)

	if len(cli.TokenFile) != 0 {
		cli.API.Token = strings.TrimSpace(string(cli.TokenFile))
	}
	k.Bind(cli.API)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		slog.Info("cancelling", slog.String("signal", (<-sig).String()))
	}()
	k.BindTo(ctx, (*context.Context)(nil))

	k.FatalIfErrorf(k.Run(ctx))
}
