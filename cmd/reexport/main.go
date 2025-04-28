// Package main contains the reexport command to reformat Firefly export data.
package main

import (
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	in := flag.String("csv", "", "CSV file path")
	acc := flag.String("acc", "", "Account ID, set as first column in output CSV")
	flag.Parse()

	file, err := os.Open(*in)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	// Account,Date,Description,Payments,Receipts,Running
	o := csv.NewWriter(os.Stdout)
	defer o.Flush()
	if err := o.Write([]string{"Account", "Date", "Description", "Payments", "Receipts", "Running"}); err != nil {
		panic(err)
	}

	// user_id,group_id,journal_id,created_at,updated_at,group_title,type,currency_code,amount,foreign_currency_code,foreign_amount,native_currency_code,native_amount,native_foreign_amount,description,date,source_name,source_iban,source_type,destination_name,destination_iban,destination_type,reconciled,category,budget,bill,tags,notes,sepa_cc,sepa_ct_op,sepa_ct_id,sepa_db,sepa_country,sepa_ep,sepa_ci,sepa_batch_id,external_url,interest_date,book_date,process_date,due_date,payment_date,invoice_date,recurrence_id,internal_reference,bunq_payment_id,import_hash,import_hash_v2,external_id,original_source,recurrence_total,recurrence_count,recurrence_date
	r := csv.NewReader(file)
	r.ReuseRecord = true
	if err := pipe(r, o, *acc); err != nil {
		panic(err)
	}
}

func pipe(r *csv.Reader, o *csv.Writer, acc string) error {
	var (
		line    int
		errs    error
		running float64
	)
	for record, err := r.Read(); err != io.EOF; record, err = r.Read() {
		line++
		if err != nil && !errors.Is(err, csv.ErrFieldCount) {
			return errors.Join(errs, err)
		}
		if len(record) < 20 {
			errs = errors.Join(errs, err)
			continue
		}
		t, err := time.Parse(time.RFC3339, record[15])
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("line %d: failed to parse time: %w", line, err))
			continue
		}
		var payments, receipts string
		f, err := strconv.ParseFloat(record[8], 64)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("line %d: failed to parse amount: %w", line, err))
			continue
		}
		if f < 0 {
			payments = fmt.Sprintf("%.2f", -f)
		} else {
			receipts = fmt.Sprintf("%.2f", f)
		}
		running += f
		if err := o.Write([]string{
			acc,
			strings.ToUpper(t.Format("02 Jan 2006")),
			record[14], // description
			payments,
			receipts,
			fmt.Sprintf("%.2f", running),
		}); err != nil {
			return err
		}
	}
	return errs
}
