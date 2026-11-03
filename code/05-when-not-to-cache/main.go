// When not to cache: walk four questions before building anything.
package main

import "fmt"

type Case struct {
	name                          string
	readOften, staleOK, slow, fix bool
	note                          string
}

// Decide walks the questions in order and stops at the first that ends it.
func Decide(c Case) string {
	switch {
	case !c.readOften:
		return "no cache: read once never pays back the copy"
	case !c.staleOK:
		return "no cache: read the source, a copy would be wrong"
	case !c.slow:
		return "no cache: the source is already fast"
	case c.fix:
		return "fix the query first: " + c.note
	}
	return "cache it, and track hit rate, source load and tail latency"
}

// Worth keeps a cache only when it hits often enough to pay for itself.
func Worth(hits, reads int) string {
	rate := 100 * float64(hits) / float64(reads)
	if rate < 80 {
		return fmt.Sprintf("hit rate %.0f%%: below about 80%%, it may cost more than it saves. Remove it.", rate)
	}
	return fmt.Sprintf("hit rate %.0f%%: keep it", rate)
}

func main() {
	for _, c := range []Case{
		{"slow graph search", true, true, true, true, "two missing indexes"},
		{"chain head height", true, false, false, false, ""},
		{"slow dashboard query", true, true, true, true, "the missing index"},
		{"costly query, index already there", true, true, true, false, ""},
	} {
		fmt.Printf("%-34s -> %s\n", c.name, Decide(c))
	}
	fmt.Println(Worth(450, 1000))
	fmt.Println(Worth(950, 1000))

	const readers = 100 // all reading the same cached query
	fmt.Printf("a cache that hides a slow query: at expiry, all %d readers run it at once\n", readers)
}
