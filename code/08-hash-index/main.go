// An in-memory model of a hash index: a real hash function, eight buckets and
// a split that doubles them. It needs no database. It counts keys compared and
// buckets read, so the numbers in the lesson come from this run.
package main

import (
	"fmt"
	"hash/fnv"
)

func hash(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	v := h.Sum32()
	return v ^ (v >> 16)
}

type Index struct {
	buckets [][]string
}

func New(n int) *Index { return &Index{buckets: make([][]string, n)} }

func (x *Index) slot(key string) int { return int(hash(key) % uint32(len(x.buckets))) }

func (x *Index) Add(key string) { s := x.slot(key); x.buckets[s] = append(x.buckets[s], key) }

// Get returns the bucket number and how many keys it compared.
func (x *Index) Get(key string) (bucket, compared int, found bool) {
	bucket = x.slot(key)
	for _, k := range x.buckets[bucket] {
		compared++
		if k == key {
			return bucket, compared, true
		}
	}
	return bucket, compared, false
}

// Grow doubles the buckets: a key of bucket b stays in b or moves to b + n.
func (x *Index) Grow() {
	old := x.buckets
	x.buckets = make([][]string, 2*len(old))
	for _, b := range old {
		for _, k := range b {
			x.Add(k)
		}
	}
}

func main() {
	x := New(8)
	for _, k := range []string{"0x8dbd", "0x6426", "0xfcc1", "0x0561", "0x764c", "0x48e5", "0x2f8b", "0xe2c9"} {
		x.Add(k)
	}
	for i, b := range x.buckets {
		fmt.Printf("bucket %d: %v\n", i, b)
	}
	b, c, ok := x.Get("0xfcc1")
	fmt.Printf("= 0xfcc1: bucket %d, compared %d, found %v\n", b, c, ok)
	b, c, ok = x.Get("0x48e5")
	fmt.Printf("= 0x48e5: bucket %d, compared %d, found %v\n", b, c, ok)

	// A range: no key tells us the bucket, so read them all.
	read, match, hit := 0, 0, map[int]bool{}
	for i, bk := range x.buckets {
		read++
		for _, k := range bk {
			if k > "0x9c00" {
				match++
				hit[i] = true
			}
		}
	}
	fmt.Printf("> 0x9c00: read %d of %d buckets, %d keys match, in %d buckets\n", read, len(x.buckets), match, len(hit))
	fmt.Printf("neighbours: 0xfcc1 goes to bucket %d, 0xfcc2 goes to bucket %d\n", hash("0xfcc1")%8, hash("0xfcc2")%8)

	x.Grow()
	fmt.Printf("after doubling to 16: 0x764c in bucket %d, 0x48e5 in bucket %d\n", x.slot("0x764c"), x.slot("0x48e5"))
	_, c, _ = x.Get("0x48e5")
	fmt.Printf("= 0x48e5 after the split: compared %d\n", c)

	// Lookups stay flat as the table grows, as long as the index keeps doubling.
	for _, n := range []int{1_000, 1_000_000} {
		y := New(n)
		for i := 0; i < n; i++ {
			y.Add(fmt.Sprintf("tx-%d", i))
		}
		total := 0
		for i := 0; i < n; i++ {
			_, c, _ := y.Get(fmt.Sprintf("tx-%d", i))
			total += c
		}
		fmt.Printf("%d keys in %d buckets: %.2f keys compared per lookup\n", n, n, float64(total)/float64(n))
	}
}
