package firefly

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/list"
	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/go-json-experiment/json"
)

type Match struct {
	AccountID       int    `short:"a" required:""`
	File            []byte `type:"filecontent" required:""`
	Start           int    `short:"s" help:"Start at row"`
	AssetIDs        []int  `name:"assets" help:"Asset account IDs create transfers"`
	KeepDescription bool   `help:"Keep existing descriptions if set" default:"true"`
	Tag             string `required:"" help:"Tag to apply to matched transactions" default:"gdpr"`
	ColDate         int    `required:"" help:"Column number for date, one-indexed"`
	DateFormat      string `required:"" help:"Format for date column" default:"02 Jan 06"`
	ColDescription  int    `required:"" help:"Column number for description"`
	ColAmount       int    `required:"" help:"Column number for amount, +deposit, -withdrawal"`
	ColWithdrawal   int    `help:"Column number for payment, if applicable"`
	ApproxTransfer  string `help:"String to find in description to approximately match transfers by month"`
}

type accountMapping map[string]int

var mapping = accountMapping{}

func (m accountMapping) match(s string) int {
	for k, v := range mapping {
		if strings.Contains(s, k) {
			return v
		}
	}
	return 0
}

func (m Match) Run(ctx context.Context, a API) error {
	c := csv.NewReader(bytes.NewReader(m.File))
	c.ReuseRecord = true
	var row int
	for record, err := c.Read(); err != io.EOF; record, err = c.Read() {
		row++
		if row < m.Start {
			continue
		}
		l := slog.With(slog.Int("row", row))
		if err != nil {
			if !errors.Is(err, csv.ErrFieldCount) {
				l.Warn("skipping record", slog.String("record", record[0]))
			}
			continue
		}

		date, err := time.Parse(m.DateFormat, record[m.ColDate-1])
		if err != nil {
			l.Warn("invalid date", slog.String("err", err.Error()), slog.String("record", record[m.ColDate-1]))
			continue
		}
		// process date → payment date
		processDate, paymentDate := date, date
		if _, r, ok := strings.Cut(record[m.ColDescription-1], " ON "); ok {
			s := strings.SplitAfterN(r, " ", 3)
			if len(s) == 3 {
				if override, err := time.Parse("02 Jan 2006", s[0]+" "+s[1]+" "+strconv.Itoa(date.Year())); err == nil {
					date, processDate = override, override
				}
			}
		}

		var (
			amount  string
			payment bool
		)
		if m.ColWithdrawal > 0 {
			amount = record[m.ColWithdrawal-1]
			if amount != "" {
				payment = true
			} else {
				amount = record[m.ColAmount-1]
			}
		} else {
			amount = record[m.ColAmount-1]
			if strings.HasPrefix(amount, "-") {
				amount = strings.TrimPrefix(amount, "-")
				payment = true
			}
		}

		formattedDate := date.Format("2006-01-02")
		if len(m.ApproxTransfer) > 0 && strings.HasPrefix(record[m.ColDescription-1], m.ApproxTransfer) {
			formattedDate = strings.Replace(formattedDate, formattedDate[8:10], "xx", 1) + ` type:"Transfer"`
		}
		var res []transactions
		q := make(url.Values, 1)
		q.Add("query", fmt.Sprintf("account_id:%d date_on:%s amount:%s -tag_is:%s", m.AccountID, formattedDate, amount, m.Tag))
		if err := Do(ctx, a, http.MethodGet, "search/transactions", q, &res, nil); err != nil {
			return err
		}

		if len(res) == 0 && !date.Equal(paymentDate) {
			l.Info("no transactions found with process date, retrying with payment date")
			q := make(url.Values, 1)
			q.Add("query", fmt.Sprintf("account_id:%d date_on:%s amount:%s -tag_is:%s", m.AccountID, paymentDate.Format("2006-01-02"), amount, m.Tag))
			if err := Do(ctx, a, http.MethodGet, "search/transactions", q, &res, nil); err != nil {
				return err
			}
		}

		title := fmt.Sprintf("%d %s %q %v %s", row, record[m.ColDate-1], record[m.ColDescription-1], payment, amount)
		l = l.With("title", title)

		var selection transaction
		switch len(res) {
		case 0:
			l.Info("no transactions found with process date (or payment date if different), asking for ID to match")
			var id int
			mappedAccountID := mapping.match(record[m.ColDescription-1])
			if id == 0 && mappedAccountID == 0 {
				id, err = askID(title)
				if err != nil {
					return err
				}
			}
			if id == 0 {
				var f StringFloat
				f.UnmarshalText([]byte(amount))
				l.Info("require opposing account ID", slog.Float64("amount", float64(f)))
				id = mappedAccountID
				if id == 0 {
					id, err = askID(title)
					if err != nil {
						return err
					}
				}
				if id == 0 {
					return errors.New("cancelling")
				}
				source, destination, t := id, m.AccountID, "deposit"
				if payment {
					source, destination, t = destination, source, "withdrawal"
				}
				if slices.Contains(m.AssetIDs, source) && slices.Contains(m.AssetIDs, destination) {
					t = "transfer"
				}
				if err := upsert(ctx, a, http.MethodPost, transaction{
					Date:          date,
					ProcessDate:   processDate,
					PaymentDate:   paymentDate,
					Type:          t,
					Description:   record[m.ColDescription-1],
					SourceID:      StringInt(source),
					DestinationID: StringInt(destination),
					Amount:        f,
					Tags:          []string{m.Tag},
				}); err != nil {
					return err
				}
				continue
			}
			var re transactions
			if err := Do(ctx, a, http.MethodGet, "transactions/"+strconv.Itoa(id), nil, &re, nil); err != nil {
				return err
			}
			res = []transactions{re}
			fallthrough

		case 1:
			l.Info("exact match")
			if len(res[0].Attributes.Transactions) != 1 {
				l.Error("target contains split, skipping", slog.Int("target", int(res[0].ID)))
				continue
			}
			res[0].Attributes.Transactions[0].topID = int(res[0].ID)
			selection = res[0].Attributes.Transactions[0]

		default:
			options := slices.Collect(func(yield func(transaction) bool) {
				for i := range res {
					for j := range res[i].Attributes.Transactions {
						res[i].Attributes.Transactions[j].topID = int(res[i].ID)
						if !yield(res[i].Attributes.Transactions[j]) {
							return
						}
					}
				}
			})
			i, err := pick(options, title)
			if err != nil {
				return err
			}
			if i < 0 {
				var f StringFloat
				f.UnmarshalText([]byte(amount))
				l.Info("require opposing account ID", slog.Float64("amount", float64(f)))
				id, err := askID(title)
				if err != nil {
					return err
				}
				if id == 0 {
					return errors.New("cancelling")
				}
				source, destination, t := id, m.AccountID, "deposit"
				if payment {
					source, destination, t = destination, source, "withdrawal"
				}
				if err := upsert(ctx, a, http.MethodPost, transaction{
					Date:          date,
					ProcessDate:   processDate,
					PaymentDate:   paymentDate,
					Type:          t,
					Description:   record[m.ColDescription-1],
					SourceID:      StringInt(source),
					DestinationID: StringInt(destination),
					Amount:        f,
					Tags:          []string{m.Tag},
				}); err != nil {
					return err
				}
				continue
			}
			selection = options[i]
		}
		l.Info("made selection", slog.String("selection", selection.String()))

		switch selection.Description {
		case "(empty description)":
			l.Info("description was empty")
			selection.Description = record[m.ColDescription-1]
		case record[m.ColDescription-1]:
			l.Info("description already matches")
		default:
			if !m.KeepDescription {
				desc, err := askText(title+" — "+selection.Description, record[m.ColDescription-1])
				if err != nil {
					return err
				}
				selection.Description = desc
			}
		}

		selection.Tags = append(selection.Tags, m.Tag)
		selection.PaymentDate = paymentDate
		selection.ProcessDate = processDate

		if err := upsert(ctx, a, http.MethodPut, selection); err != nil {
			return err
		}
	}

	return nil
}

func pick(options []transaction, title string) (int, error) {
	l := list.New(make([]list.Item, len(options)), itemDelegate{options}, 73, min(len(options)+6, 10))
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	m := listModel{Model: l}
	_, err := tea.NewProgram(&m).Run()
	return m.int, err
}

func askID(title string) (int, error) {
	fmt.Println(title)
	input := textinput.New()
	input.Validate = func(s string) error {
		_, err := strconv.Atoi(s)
		return err
	}
	input.Focus()
	input.Placeholder = "ID"
	input.SetWidth(73)
	m := textModel{Model: input}
	_, err := tea.NewProgram(&m).Run()
	if err != nil {
		if m.Value() == "" {
			return 0, nil
		}
		return 0, err
	}
	return strconv.Atoi(m.Value())
}

func askText(title, value string) (string, error) {
	fmt.Println(title)
	input := textinput.New()
	input.Focus()
	input.Placeholder = ""
	input.SetValue(value)
	input.SetWidth(73)
	m := textModel{Model: input}
	_, err := tea.NewProgram(&m).Run()
	if err != nil {
		if m.Value() == "" {
			return "", nil
		}
		return "", err
	}
	return m.Value(), nil
}

func upsert(ctx context.Context, a API, method string, t transaction) error {
	json.MarshalWrite(os.Stdout, t)
	io.WriteString(os.Stdout, "\n")
	body, err := json.Marshal(transactionUpdate{
		Transactions: []transaction{t},
	})
	if err != nil {
		return err
	}
	var out any
	path := "transactions"
	if method == http.MethodPut {
		path += "/" + strconv.Itoa(int(t.topID))
	}
	if err := Do(ctx, a, method, path, nil, &out, bytes.NewReader(body)); err != nil {
		return err
	}
	json.MarshalWrite(os.Stdout, out)
	io.WriteString(os.Stdout, "\n")
	return nil
}

type listModel struct {
	list.Model
	int
}

func (m listModel) Init() tea.Cmd { return nil }
func (m *listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if cmd := keypress(msg); cmd != nil {
		// workaround global index being 0 after exit
		m.int = m.Model.GlobalIndex()
		return m, cmd
	}
	var cmd tea.Cmd
	m.Model, cmd = m.Model.Update(msg)
	return m, cmd
}

type textModel struct{ textinput.Model }

func (m textModel) Init() tea.Cmd { return textinput.Blink }
func (m *textModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if cmd := keypress(msg); cmd != nil {
		return m, cmd
	}
	var cmd tea.Cmd
	m.Model, cmd = m.Model.Update(msg)
	return m, cmd
}

func keypress(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch keypress := key.String(); keypress {
		case "enter":
			return tea.Quit
		case "q", "ctrl+c":
			return tea.Interrupt
		}
	}
	return nil
}

type transactionUpdate struct {
	Transactions []transaction `json:"transactions"`
}

type transaction struct {
	ID            StringInt   `json:"transaction_journal_id,omitzero"`
	Date          time.Time   `json:"date"`
	ProcessDate   time.Time   `json:"process_date,omitzero"` // start
	PaymentDate   time.Time   `json:"payment_date,omitzero"` // end
	Type          string      `json:"type"`
	Description   string      `json:"description"`
	Source        string      `json:"source_name,omitzero"`
	SourceID      StringInt   `json:"source_id,omitzero"`
	Destination   string      `json:"destination_name,omitzero"`
	DestinationID StringInt   `json:"destination_id,omitzero"`
	Amount        StringFloat `json:"amount"`
	Tags          []string    `json:"tags,omitzero"`

	topID int
}

func (t transaction) String() string {
	return fmt.Sprintf("%d %s %q (%s → %s) %.2f", t.ID, t.Date.Format("02 Jan 2006"), t.Description, t.Source, t.Destination, t.Amount)
}

type zeroItem struct{}

// FilterValue implements [list.Item].
func (t zeroItem) FilterValue() string { return "" }

type transactions struct {
	ID         StringInt         `json:"id"`
	Attributes transactionUpdate `json:"attributes"`
}

type simpleItem string

var _ list.DefaultItem = (*simpleItem)(nil)

func (s simpleItem) FilterValue() string { return string(s) }
func (s simpleItem) Title() string       { return string(s) }
func (s simpleItem) Description() string { return "" }

type itemDelegate struct{ options []transaction }

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, i int, _ list.Item) {
	dd := list.NewDefaultDelegate()
	dd.ShowDescription = false
	dd.Render(w, m, i, simpleItem(d.options[i].String()))
}
