// A real bloom filter, in memory, over SHA-256. It needs no database. The story
// keys were picked so their bit positions are the ones the lesson draws.
package main

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

type Bloom struct {
	bits []bool
	k    int
}

func New(m, k int) *Bloom { return &Bloom{bits: make([]bool, m), k: k} }

// positions runs the k hash functions: SHA-256 of the key and a function number.
func (b *Bloom) positions(key string) []int {
	out := make([]int, 0, b.k)
	for i := 0; i < b.k; i++ {
		s := sha256.Sum256([]byte(fmt.Sprintf("%s#%d", key, i)))
		// One byte is enough for the 16-bit story filter. Bigger filters use 4 bytes.
		var v uint32
		if len(b.bits) <= 256 {
			v = uint32(s[0])
		} else {
			v = binary.BigEndian.Uint32(s[:4])
		}
		out = append(out, int(v%uint32(len(b.bits))))
	}
	return out
}

func (b *Bloom) Add(key string) {
	for _, p := range b.positions(key) {
		b.bits[p] = true
	}
}

// Maybe is false only when a bit is 0: then the key was surely never added.
func (b *Bloom) Maybe(key string) bool {
	for _, p := range b.positions(key) {
		if !b.bits[p] {
			return false
		}
	}
	return true
}

// declaring one in StarRocks:
// ALTER TABLE swaps SET ("bloom_filter_columns" = "tx_hash");

func main() {
	f := New(16, 3)
	aa, bb, c9, s77 := "0xaa0010", "0xbb02f6", "0x9c4100b0", "0x7700a4"
	f.Add(aa)
	f.Add(bb)
	fmt.Printf("add %s: bits %v\n", aa, f.positions(aa))
	fmt.Printf("add %s: bits %v\n", bb, f.positions(bb))
	for _, k := range []string{c9, aa, s77} {
		fmt.Printf("look up %s: bits %v, maybe = %v\n", k, f.positions(k), f.Maybe(k))
	}

	// Eight blocks. Block 3 holds aa. Block 7 holds two other keys that happen to
	// set aa's bits. The other six hold one key each.
	blocks := make([]*Bloom, 8)
	for i := range blocks {
		blocks[i] = New(16, 3)
		blocks[i].Add(fmt.Sprintf("0xd%d", i))
	}
	blocks[2].Add(aa)
	blocks[6].Add("0xc10498")
	blocks[6].Add("0xc2007d")
	read := 0
	for i, b := range blocks {
		if b.Maybe(aa) {
			read++
			fmt.Printf("  block %d says maybe\n", i+1)
		}
	}
	fmt.Printf("look up %s across the day: %d of 8 blocks read\n", aa, read)

	// Saturated: thousands of keys in 16 bits.
	full := New(16, 3)
	for i := 0; i < 5000; i++ {
		full.Add(fmt.Sprintf("tx-%d", i))
	}
	on := 0
	for _, x := range full.bits {
		if x {
			on++
		}
	}
	fmt.Printf("5000 keys in 16 bits: %d bits set, %s maybe = %v\n", on, c9, full.Maybe(c9))

	// Sized right: about 10 bits per key, with 7 hash functions.
	const keys, probes = 100_000, 100_000
	sized := New(keys*10, 7)
	for i := 0; i < keys; i++ {
		sized.Add(fmt.Sprintf("in-%d", i))
	}
	falseAlarms := 0
	for i := 0; i < probes; i++ {
		if sized.Maybe(fmt.Sprintf("out-%d", i)) {
			falseAlarms++
		}
	}
	missed := 0
	for i := 0; i < keys; i++ {
		if !sized.Maybe(fmt.Sprintf("in-%d", i)) {
			missed++
		}
	}
	fmt.Printf("10 bits per key: %.2f%% false alarms, %d real keys missed\n", 100*float64(falseAlarms)/probes, missed)
}
