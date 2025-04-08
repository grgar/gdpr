package main

import (
	"bytes"
	crand "crypto/rand"
	"encoding/csv"
	"fmt"
	"math/rand/v2"
	"testing"
	"time"
)

func BenchmarkParse(b *testing.B) {
	const lines = 1_000_000
	var buf bytes.Buffer
	fmt.Fprintln(&buf, "12-34-56", "12345678")
	for range lines {
		if rand.IntN(3) == 0 {
			fmt.Fprintln(&buf, crand.Text(), " | | | | ||")
		}
		fmt.Fprintf(&buf, "%s | ", crand.Text())
		val := rand.Float64()*200 - 100 // Random value between -100 and 100
		if val < 0 {
			fmt.Fprintf(&buf, "%.2f | | ", -val)
		} else {
			fmt.Fprintf(&buf, "| %.2f | ", val)
		}
		fmt.Fprint(&buf, time.Now().Format("02 JAN 2006"), " | ")
		fmt.Fprintf(&buf, "%.2f || %.2f\n", rand.Float32(), rand.Float32())
	}
	var buf2 bytes.Buffer
	w := csv.NewWriter(&buf2)

	for b.Loop() {
		parse(&buf, w)
	}
	w.Flush()
}
