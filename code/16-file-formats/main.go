// File formats: Avro rows in motion, Parquet columns at rest, JSON at the
// edge, and a schema change that renames a field. Replays the lesson.
package main

import (
	"fmt"
	"sort"
	"strconv"
)

type Swap struct {
	Trader, Pool string
	Amount       float64
	Day          string
}

var swaps = []Swap{
	{"0xab..", "P1", 12.5, "3 Oct"}, {"0x11..", "P2", 3.0, "3 Oct"}, {"0x5c..", "P1", 40, "3 Oct"},
	{"0x90..", "P3", 7.25, "3 Oct"}, {"0xab..", "P2", 1.5, "3 Oct"},
}

const fields = 4

// AvroSumAmount reads row after row: every field of every record comes off disk.
func AvroSumAmount(rows []Swap) (sum float64, cells int) {
	for _, r := range rows {
		cells += fields // the whole record is decoded
		sum += r.Amount
	}
	return sum, cells
}

// ParquetFile keeps each column apart, with min and max per column block.
type ParquetFile struct {
	Amount         []float64
	MinAmt, MaxAmt float64
}

func ToParquet(rows []Swap) ParquetFile {
	p := ParquetFile{MinAmt: rows[0].Amount, MaxAmt: rows[0].Amount}
	for _, r := range rows { // write each column's values together
		p.Amount = append(p.Amount, r.Amount)
		p.MinAmt, p.MaxAmt = min(p.MinAmt, r.Amount), max(p.MaxAmt, r.Amount)
	}
	return p
}

// ParquetSumAmount reads only the amount column.
func ParquetSumAmount(p ParquetFile) (sum float64, cells int) {
	for _, a := range p.Amount {
		cells++
		sum += a
	}
	return sum, cells
}

// DayFile is one day of swaps written with the schema of its day.
type DayFile struct {
	Day    int
	Fields map[string]float64 // field name -> that day's total
}

// SumField applies the schema on read: a missing field reads as null (0).
func SumField(files []DayFile, name string) (sum float64, nulls int) {
	for _, f := range files {
		v, ok := f.Fields[name]
		if !ok {
			nulls++ // no such field in this file: null, no error
			continue
		}
		sum += v
	}
	return sum, nulls
}

// Compatible is the registry check: a new version may add fields with a
// default, but must keep every old field.
func Compatible(old, next []string) bool {
	have := map[string]bool{}
	for _, f := range next {
		have[f] = true
	}
	for _, f := range old {
		if !have[f] {
			return false // removed or renamed: old readers would break
		}
	}
	return true
}

func main() {
	sum, cells := AvroSumAmount(swaps)
	fmt.Printf("Avro:    SUM(amount) = %.2f, cells read %d\n", sum, cells)
	p := ToParquet(swaps)
	sum, cells = ParquetSumAmount(p)
	fmt.Printf("Parquet: SUM(amount) = %.2f, cells read %d, block min %.1f max %.0f\n", sum, cells, p.MinAmt, p.MaxAmt)

	// JSON from a partner: amounts arrive as text and sort as text
	text := []string{"9", "100"}
	sort.Strings(text)
	fmt.Println("JSON sorted as text:", text)
	nums := []float64{}
	for _, t := range text {
		n, _ := strconv.ParseFloat(t, 64) // the fix: parse into a typed field on landing
		nums = append(nums, n)
	}
	sort.Float64s(nums)
	fmt.Println("parsed and sorted as numbers:", nums)

	// schema change: days 1-3 use amount, days 4-5 renamed it to amount_in
	var files []DayFile
	for d := 1; d <= 5; d++ {
		name := "amount"
		if d >= 4 {
			name = "amount_in"
		}
		files = append(files, DayFile{d, map[string]float64{name: 10}})
	}
	total, nulls := SumField(files, "amount")
	fmt.Printf("rename: SUM(amount) over 5 days = %.0f (right answer 50), %d days read as null\n", total, nulls)
	v1 := []string{"trader", "pool", "amount"}
	fmt.Println("registry: rename amount -> amount_in compatible?", Compatible(v1, []string{"trader", "pool", "amount_in"}))
	fmt.Println("registry: add fee with default 0 compatible?", Compatible(v1, []string{"trader", "pool", "amount", "fee"}))
}
