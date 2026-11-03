// Negative caching: remember NOT FOUND for a short time.
// A bot polls one fresh address every second for a minute.
package main

import "fmt"

const notFound = "NOT FOUND"

type entry struct {
	val     string
	expires int // seconds
}

type Service struct {
	graph       map[string]string // what the graph database knows
	cache       map[string]entry
	lookups     int
	negative    bool // store NOT FOUND markers
	positiveTTL int
	negativeTTL int
}

func (s *Service) Lookup(addr string, now int) string {
	if e, ok := s.cache[addr]; ok && now < e.expires {
		return e.val // a hit, also for a NOT FOUND marker
	}
	s.lookups++ // a full graph search
	v, ok := s.graph[addr]
	if !ok {
		if s.negative {
			s.cache[addr] = entry{notFound, now + s.negativeTTL} // short TTL
		}
		return notFound
	}
	s.cache[addr] = entry{v, now + s.positiveTTL}
	return v
}

// AddAddress is the graph sync. The fix deletes any marker for the new address.
func (s *Service) AddAddress(addr, v string, deleteMarker bool) {
	s.graph[addr] = v
	if deleteMarker {
		delete(s.cache, addr)
	}
}

func newService(negative bool) *Service {
	return &Service{graph: map[string]string{}, cache: map[string]entry{}, negative: negative, positiveTTL: 300, negativeTTL: 30}
}

func main() {
	const addr = "0x5e..aa01"
	for _, neg := range []bool{false, true} {
		s := newService(neg)
		for now := 0; now < 60; now++ { // the bot, every second
			s.Lookup(addr, now)
		}
		fmt.Printf("negative caching=%v: 60 requests in a minute -> %d graph lookups\n", neg, s.lookups)
	}

	fmt.Println("\nthe address gets its first transaction at second 65:")
	for _, fix := range []bool{false, true} {
		s := newService(true)
		s.Lookup(addr, 60) // marker stored until second 90
		s.AddAddress(addr, "fresh wallet, 1 transaction", fix)
		fmt.Printf("  delete marker on create=%v: at second 66 the investigator sees %q\n", fix, s.Lookup(addr, 66))
	}
}
