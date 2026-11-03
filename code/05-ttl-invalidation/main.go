// TTL, event invalidation and a version in the key, on one story: an analyst
// marks 0x9f..0071 as Scam, and three cached risk answers used the old label.
package main

import "fmt"

const (
	scammer = "0x9f..0071"
	senderA = "0x77..30ff" // sent money to the scammer
	senderB = "0xc2..5518" // sent money to the scammer
)

type item struct {
	answer  string
	expires int
}

type Service struct {
	label         string // the label of the scammer address in the graph
	labelsVersion int
	versionInKey  bool
	cache         map[string]item
	ttl           func(key string) int
	computed      int
}

func (s *Service) key(addr string) string {
	if s.versionInKey {
		return fmt.Sprintf("risk:%s:v%d", addr, s.labelsVersion)
	}
	return "risk:" + addr
}

// Risk reads the cache, or computes the answer from the graph.
func (s *Service) Risk(addr string, now int) string {
	k := s.key(addr)
	if it, ok := s.cache[k]; ok && now < it.expires {
		return it.answer // may be stale, for at most one TTL
	}
	s.computed++
	answer := "low risk"
	if s.label == "Scam" { // all three answers depend on this label
		answer = "high risk"
	}
	s.cache[k] = item{answer, now + s.ttl(k)}
	return answer
}

// ChangeLabel saves the label and raises the labels version.
func (s *Service) ChangeLabel(label string) {
	s.label = label
	s.labelsVersion++
}

// OnChanged is the event handler: it can only drop the key that names the address.
func (s *Service) OnChanged(addr string) {
	delete(s.cache, s.key(addr))
}

func newService(versionInKey bool) *Service {
	s := &Service{labelsVersion: 7, versionInKey: versionInKey, cache: map[string]item{}}
	s.ttl = func(string) int { return 60 }
	for _, a := range []string{scammer, senderA, senderB} {
		s.Risk(a, 0)
	}
	return s
}

func main() {
	fmt.Println("== TTL 60 s ==")
	s := newService(false)
	s.ChangeLabel("Scam") // at second 10
	fmt.Println("  second 30:", s.Risk(senderA, 30), "(stale)")
	fmt.Println("  second 61:", s.Risk(senderA, 61), "(the TTL ran out, fresh)")

	fmt.Println("== jitter: keys set together do not expire together ==")
	jitter := map[string]int{"risk:" + scammer: 57, "risk:" + senderA: 61, "risk:" + senderB: 64}
	s = &Service{cache: map[string]item{}}
	s.ttl = func(k string) int { return jitter[k] }
	for _, a := range []string{scammer, senderA, senderB} {
		s.Risk(a, 0)
		fmt.Printf("  %s expires at second %d\n", a, s.cache[s.key(a)].expires)
	}

	fmt.Println("== event: drop the key that names the address ==")
	s = newService(false)
	s.ChangeLabel("Scam")
	s.OnChanged(scammer)
	fmt.Printf("  %s: %s (key dropped)\n", scammer, s.Risk(scammer, 10))
	fmt.Printf("  %s: %s (hidden dependent, key not dropped)\n", senderA, s.Risk(senderA, 10))

	fmt.Println("== version in the key ==")
	s = newService(true)
	before := s.computed
	s.ChangeLabel("Scam")
	fmt.Printf("  key is now %s: %s\n", s.key(senderA), s.Risk(senderA, 10))
	fmt.Printf("  %s: %s\n", senderB, s.Risk(senderB, 10))
	fmt.Printf("  %s: %s\n", scammer, s.Risk(scammer, 10))
	fmt.Printf("  the cost: %d answers recomputed by one label change\n", s.computed-before)
	fmt.Printf("  cache entries: %d, the old v7 keys wait for their TTL\n", len(s.cache))
}
