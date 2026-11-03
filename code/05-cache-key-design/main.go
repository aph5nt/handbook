// Cache key design: equal requests must build equal keys, and every input
// that changes the answer must be in the key.
package main

import (
	"fmt"
	"strings"
)

const schema = "v4" // bumped from v3 when the risk model changed

type Request struct {
	Network, Address string
	LabelsVersion    int
	RequestID        string
}

// badKey: only the address, exactly as typed.
func badKey(r Request) string { return r.Address }

// goodKey: normalise, then include every input that changes the answer.
func goodKey(r Request) string {
	return fmt.Sprintf("risk:%s:%s:%s:labels%d",
		schema, r.Network, strings.ToLower(r.Address), r.LabelsVersion)
}

// debugKey: a request id splits the cache: every key is unique.
func debugKey(r Request) string { return goodKey(r) + ":" + r.RequestID }

type Cache struct {
	data         map[string]string
	hits, misses int
}

func (c *Cache) Get(key string, load func() string) string {
	if v, ok := c.data[key]; ok {
		c.hits++
		return v
	}
	c.misses++
	v := load()
	c.data[key] = v
	return v
}

func main() {
	answer := func(r Request) func() string {
		return func() string { return "risk on " + r.Network }
	}
	reqs := []Request{
		{"robinhood", "0x9F..0071", 7, "a"},
		{"robinhood", "0x9f..0071", 7, "b"}, // the same address, lowercase
		{"base", "0x9f..0071", 7, "c"},      // the same address on another chain
	}
	for _, kb := range []struct {
		name string
		key  func(Request) string
	}{{"address only", badKey}, {"normalised, every input", goodKey}, {"with a request id", debugKey}} {
		c := &Cache{data: map[string]string{}}
		fmt.Printf("== %s ==\n", kb.name)
		for _, r := range reqs {
			fmt.Printf("  %-9s %s -> key %-44q answer %q\n", r.Network, r.Address, kb.key(r), c.Get(kb.key(r), answer(r)))
		}
		fmt.Printf("  entries=%d hits=%d misses=%d\n", len(c.data), c.hits, c.misses)
	}
}
