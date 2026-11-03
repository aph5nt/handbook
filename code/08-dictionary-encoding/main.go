// A real dictionary encoder, in memory, over one segment of a column. It needs
// no database. It counts bytes, so the sizes in the lesson come from this run.
package main

import (
	"fmt"
	"math/bits"
)

type Segment struct {
	dict  []string       // code to value
	index map[string]int // value to code
	codes []int
}

func Encode(col []string) *Segment {
	s := &Segment{index: map[string]int{}}
	for _, v := range col {
		c, ok := s.index[v]
		if !ok {
			c = len(s.dict)
			s.index[v] = c
			s.dict = append(s.dict, v)
		}
		s.codes = append(s.codes, c)
	}
	return s
}

// bitsPerCode is the width that holds every code of this dictionary.
func (s *Segment) bitsPerCode() int {
	if len(s.dict) <= 1 {
		return 1
	}
	return bits.Len(uint(len(s.dict) - 1))
}

func (s *Segment) encodedBytes() int {
	n := (len(s.codes)*s.bitsPerCode() + 7) / 8
	for _, v := range s.dict {
		n += len(v)
	}
	return n
}

func rawBytes(col []string) int {
	n := 0
	for _, v := range col {
		n += len(v)
	}
	return n
}

// Filter translates the constant once, then compares integers.
func (s *Segment) Filter(value string) (rows []int, skipped bool) {
	code, ok := s.index[value]
	if !ok {
		return nil, true // not in the dictionary: skip the whole segment
	}
	for i, c := range s.codes {
		if c == code {
			rows = append(rows, i)
		}
	}
	return rows, false
}

func (s *Segment) GroupCount() map[string]int {
	counts := make([]int, len(s.dict))
	for _, c := range s.codes {
		counts[c]++
	}
	out := map[string]int{}
	for c, n := range counts {
		out[s.dict[c]] = n // decode last, one string per group
	}
	return out
}

func main() {
	col := []string{"aerodrome", "uniswap", "aerodrome", "aerodrome", "velodrome", "uniswap", "aerodrome", "curve"}
	s := Encode(col)
	fmt.Printf("dictionary %v\ncodes %v, %d bits per code\n", s.dict, s.codes, s.bitsPerCode())
	rows, _ := s.Filter("aerodrome")
	fmt.Printf("dex_name = aerodrome: code %d, rows %v\n", s.index["aerodrome"], rows)
	_, skipped := s.Filter("pancake")
	fmt.Printf("dex_name = pancake: segment skipped = %v\n", skipped)
	fmt.Printf("group by: %v\n", s.GroupCount())

	const n = 1_000_000
	few := make([]string, n)
	names := []string{"aerodrome", "uniswap-v3", "velodrome", "curve", "pancake", "balancer"}
	for i := range few {
		few[i] = names[i%len(names)]
	}
	many := make([]string, n)
	for i := range many {
		many[i] = fmt.Sprintf("0x%064x", i) // a tx hash: 66 characters
	}
	for _, c := range []struct {
		name string
		col  []string
	}{{"dex_name", few}, {"tx_hash", many}} {
		e := Encode(c.col)
		r, b := rawBytes(c.col), e.encodedBytes()
		choice := "dictionary"
		if b >= r {
			choice = "plain"
		}
		fmt.Printf("%-9s %d rows, %d distinct: raw %d bytes, encoded %d bytes, %d bits per code: store %s\n",
			c.name, n, len(e.dict), r, b, e.bitsPerCode(), choice)
	}
}
