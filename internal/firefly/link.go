package firefly

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-json-experiment/json"
)

type Link struct {
	Query string `short:"q" help:"Query for transactions containing notes" required:""`
	Input []byte `short:"i" help:"Substitute input" hidden:"" type:"filecontent"`
}

func (l Link) Run(ctx context.Context, a API) error {
	q := make(url.Values, 1)
	q.Add("query", l.Query)
	q.Add("limit", "65536")

	var resp []struct {
		Attributes struct {
			Transactions []struct {
				ID         StringInt `json:"transaction_journal_id"`
				ExternalID StringInt `json:"external_id"`
				Notes      string    `json:"notes"`
			} `json:"transactions"`
		} `json:"attributes"`
	}
	if len(l.Input) > 0 {
		ctx = context.WithValue(ctx, OverrideReaderContextKey, bytes.NewReader(l.Input))
	}
	if err := Do(ctx, a, "search/transactions", http.MethodGet, q, &resp, nil); err != nil {
		return err
	}
	ctx = context.WithValue(ctx, OverrideReaderContextKey, nil)

	for _, r := range resp {
		for _, t := range r.Attributes.Transactions {
			lhs, note, ok := strings.Cut(t.Notes, " ")
			if !ok {
				continue
			}
			if lhs == "0.0" {
				slog.Warn("expected 0% split", slog.Int("id", int(t.ID)), slog.String("note", note))
			}
			note = strings.TrimSpace(note)
			for dst := range strings.SplitSeq(note, "|") {
				slog.Info("link", slog.Int("id", int(t.ID)), slog.String("destination external", dst))

				var resp []struct {
					Attributes struct {
						Transactions []struct {
							ID StringInt `json:"transaction_journal_id"`
						} `json:"transactions"`
					} `json:"attributes"`
				}
				q := make(url.Values, 1)
				q.Add("query", `external_id_is:`+dst)
				q.Add("limit", "1")
				if err := Do(ctx, a, "search/transactions", http.MethodGet, q, &resp, nil); err != nil {
					return err
				}
				if len(resp) == 0 {
					slog.Error("no transaction found", slog.Int("id", int(t.ID)), slog.String("destination external", dst))
					continue
				}

				p, err := json.Marshal(struct {
					Type int `json:"link_type_id"`
					From int `json:"inward_id"`
					To   int `json:"outward_id"`
				}{
					Type: 3,
					From: int(resp[0].Attributes.Transactions[0].ID),
					To:   int(t.ID),
				})
				if err != nil {
					return err
				}
				var out any
				slog.Info("creating link", slog.String("payload", string(p)))
				if err := Do(ctx, a, http.MethodPost, "transaction-links", nil, &out, bytes.NewReader(p)); err != nil {
					slog.Error("failed to create link", slog.Int("id", int(t.ID)), slog.String("err", err.Error()))
					continue
				}
			}
		}
	}

	return nil
}
