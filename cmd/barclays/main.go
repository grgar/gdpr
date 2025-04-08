package main

import (
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func main() {
	file := flag.String("f", "", "file to read")
	flag.Parse()
	if *file == "" {
		flag.Usage()
		os.Exit(1)
	}
	f, err := os.Open(*file)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	var (
		sortCodeMatcher         = regexp.MustCompile(`\d{6}`)
		accountNumberMatcher    = regexp.MustCompile(`\d{4}-\d{4}`)
		sortCode, accountNumber int
		desc, account           string
	)

	w := csv.NewWriter(os.Stdout)
	defer w.Flush()

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
					panic(err)
				}
				fallthrough
			case accountNumber == 0 && accountNumberMatcher.MatchString(line):
				match := accountNumberMatcher.FindString(line)
				if match == "" {
					continue
				}
				accountNumber, err = strconv.Atoi(match[:4] + match[5:])
				if err != nil {
					panic(err)
				}
			}
			continue
		}
		if err != nil {
			panic(err)
		}
		if sortCode == 0 || accountNumber == 0 {
			panic(fmt.Errorf("sort code %d or account number %d not found", sortCode, accountNumber))
		} else if account == "" {
			account = fmt.Sprintf("%06d %08d", sortCode, accountNumber)
		}

		details, payments, receipts, date := strings.TrimSpace(record[0]), strings.TrimSpace(record[1]), strings.TrimRight(record[2], "C "), strings.TrimSpace(record[3])
		if payments == "" && receipts == "" {
			desc = details
			continue
		}
		description := details
		if desc != "" {
			description = desc + " " + description
		}
		if err := w.Write([]string{account, date, description, payments, receipts}); err != nil {
			panic(err)
		}
		desc = ""
	}
}
