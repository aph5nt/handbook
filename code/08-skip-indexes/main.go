// An in-memory model of skip indexes over one day of swaps: zone maps per block
// and bitmaps per value. It needs no database. The bloom filter of the second
// tab has its own program, handbook/code/08-bloom-filter.
package main

import "fmt"

type Block struct {
	MinPool, MaxPool     int
	MinHeight, MaxHeight int
	MinHash, MaxHash     int // tx_hash as a number, 0x00 to 0xff
}

// zone is the only check a zone map can run: can this range hold the value?
func zone(lo, hi, v int) bool { return v >= lo && v <= hi }

func bitAnd(a, b string) (string, int) {
	out, n := "", 0
	for i := range a {
		if a[i] == '1' && b[i] == '1' {
			out += "1"
			n++
		} else {
			out += "0"
		}
	}
	return out, n
}

func main() {
	// Sorted by (pool, block_height), so pool ranges do not overlap, except where
	// one pool crosses a block edge: pool 431 is in blocks 5 and 6.
	day := []Block{
		{1, 120, 0, 0, 0x00, 0xff}, {121, 240, 0, 0, 0x00, 0xff}, {241, 360, 0, 0, 0x00, 0xff},
		{361, 430, 0, 0, 0x00, 0xff}, {431, 431, 21_300_000, 21_399_000, 0x00, 0xff},
		{431, 520, 21_399_500, 21_800_000, 0x00, 0xff},
		{521, 700, 0, 0, 0x00, 0xff}, {701, 999, 0, 0, 0x00, 0xff},
	}
	read := []int{}
	for i, b := range day {
		if zone(b.MinPool, b.MaxPool, 431) {
			read = append(read, i+1)
		}
	}
	fmt.Printf("pool = 431: blocks read %v, %d of 8\n", read, len(read))

	read = read[:0]
	for i, b := range day {
		if zone(b.MinPool, b.MaxPool, 431) && b.MaxHeight > 21_400_000 {
			read = append(read, i+1)
		}
	}
	fmt.Printf("pool = 431 AND height > 21,400,000: blocks read %v, %d of 8\n", read, len(read))

	n := 0
	for _, b := range day {
		if zone(b.MinHash, b.MaxHash, 0x9c) {
			n++
		}
	}
	fmt.Printf("tx_hash = 0x9c41...: %d of 8 ranges can hold it, 0 skipped\n", n)

	aero, weth := "1001100100110010", "1101001011010001"
	both, count := bitAnd(aero, weth)
	fmt.Printf("bitmap aerodrome %s\nbitmap WETH      %s\nAND              %s: %d of 16 rows read\n", aero, weth, both, count)

	const rows, values = 1_000_000, 1_000_000
	fmt.Printf("bitmap on address: %d values x %d bits = %d GB before compression\n",
		values, rows, values*rows/8/1_000_000_000)
}
