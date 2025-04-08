package main

import (
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	sortCodeMatcher      = regexp.MustCompile(`\d{6}`)
	accountNumberMatcher = regexp.MustCompile(`\d{4}-\d{4}`)
)

func main() {
	flag.Usage = func() { fmt.Fprintf(os.Stderr, "Usage: %s <file> [<file>…]\n", filepath.Base(os.Args[0])) }
	flag.Parse()
	files := flag.Args()
	if len(files) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	w := csv.NewWriter(os.Stdout)
	defer w.Flush()
	if err := w.Write([]string{"Account", "Date", "Description", "Payments", "Receipts", "Running"}); err != nil {
		panic(err)
	}

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			panic(err)
		}
		parse(f, w)
		f.Close()
	}
}

func parse(f io.ReadCloser, w *csv.Writer) error {
	var (
		sortCode, accountNumber int
		desc, account           string
	)

	r := csv.NewReader(f)
	r.Comma = '|'
	r.FieldsPerRecord = 7
	r.ReuseRecord = true
	r.TrimLeadingSpace = true
	for record, err := r.Read(); err != io.EOF; record, err = r.Read() {
		if errors.Is(err, csv.ErrFieldCount) {
			line := record[0]
			switch {
			case sortCode == 0 && sortCodeMatcher.MatchString(line):
				match := sortCodeMatcher.FindString(line)
				if match == "" {
					continue
				}
				sortCode, err = strconv.Atoi(match)
				if err != nil {
					return err
				}
				fallthrough
			case accountNumber == 0 && accountNumberMatcher.MatchString(line):
				match := accountNumberMatcher.FindString(line)
				if match == "" {
					continue
				}
				accountNumber, err = strconv.Atoi(match[:4] + match[5:])
				if err != nil {
					return err
				}
			}
			continue
		}
		if err != nil {
			return err
		}
		if sortCode == 0 || accountNumber == 0 {
			return fmt.Errorf("sort code %d or account number %d not found", sortCode, accountNumber)
		} else if account == "" {
			account = fmt.Sprintf("%06d %08d", sortCode, accountNumber)
		}

		details, payments, receipts, date, running := strings.TrimSpace(record[0]), strings.TrimSpace(record[1]), strings.TrimRight(record[2], "C "), strings.TrimSpace(record[3]), strings.TrimSpace(record[4])
		if payments == "" && receipts == "" {
			desc = details
			continue
		}
		if payments != "" {
			if _, err := strconv.ParseFloat(payments, 64); err != nil {
				continue
			}
		}
		if receipts != "" {
			if _, err := strconv.ParseFloat(receipts, 64); err != nil {
				continue
			}
		}
		description := details
		if desc != "" {
			description = desc + " " + description
		}
		if err := w.Write([]string{account, date, description, payments, receipts, running}); err != nil {
			return err
		}
		desc = ""
	}
	return nil
}
