// Command eureka replays the Eureka lesson: registration, lease renewal by
// heartbeat, eviction after 90 s, self-preservation when renewals drop under
// 85 %, a client-side cache, and a client retry on a stale entry.
// Profile runs 20 copies; the lesson draws copy 3 and copy 7.
package main

import (
	"fmt"
	"sort"
)

type Registry struct {
	Leases           map[string]int // copy -> second of last renewal
	Renews           []int          // seconds at which heartbeats arrived
	SelfPreservation bool
}

func (r *Registry) Register(copy string, now int) { r.Leases[copy] = now }

func (r *Registry) Heartbeat(copy string, now int) {
	r.Leases[copy] = now
	r.Renews = append(r.Renews, now)
}

func (r *Registry) Evict(now int) (evicted []string) {
	expected := len(r.Leases) * 2
	got := 0
	for _, t := range r.Renews {
		if t > now-60 {
			got++
		}
	}
	if float64(got) < 0.85*float64(expected) {
		r.SelfPreservation = true
		return nil
	}
	r.SelfPreservation = false
	for copy, last := range r.Leases {
		if now-last > 90 {
			delete(r.Leases, copy)
			evicted = append(evicted, copy)
		}
	}
	return evicted
}

// Fetch is what each client copies into its cache every 30 s.
func (r *Registry) Fetch() []string { return r.list() }

// Call tries each cached copy until one answers.
func Call(cache []string, alive map[string]bool) (tried []string) {
	for _, c := range cache {
		tried = append(tried, c)
		if alive[c] {
			return tried
		}
	}
	return tried
}

func (r *Registry) list() []string {
	var out []string
	for c := range r.Leases {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

func clock(s int) string { return fmt.Sprintf("10:%02d:%02d", s/60, s%60) }

func main() {
	r := &Registry{Leases: map[string]int{}}
	alive := map[string]bool{}
	copies := []string{}
	for i := 1; i <= 20; i++ {
		c := fmt.Sprintf("copy%02d", i)
		copies = append(copies, c)
		alive[c] = true
		r.Register(c, 0)
	}
	fmt.Println("step 1: 20 Profile copies registered at 10:00:00, including copy03 and copy07")
	beat := func(from, to int, skip func(c string, t int) bool) {
		for t := from; t <= to; t += 30 {
			for _, c := range copies {
				if alive[c] && !skip(c, t) {
					r.Heartbeat(c, t)
				}
			}
		}
	}
	none := func(string, int) bool { return false }
	beat(30, 270, none)
	fmt.Println("step 2: heartbeats every 30 s renew each lease")
	fmt.Println("step 3: client cache holds", len(r.Fetch()), "copies")
	fmt.Println("step 4: call order [copy07 copy03] -> tried", Call([]string{"copy07", "copy03"}, alive))

	alive["copy03"] = false // dies at 10:05:00, last renewal 10:04:30
	for t := 300; t <= 420; t += 60 {
		beat(t-30, t, func(c string, s int) bool { return s <= 270 })
		ev := r.Evict(t)
		got := 0
		for _, s := range r.Renews {
			if s > t-60 {
				got++
			}
		}
		fmt.Printf("step 5: %s evict check: renewals %d of %d, last copy03 renewal %s, evicted %v\n",
			clock(t), got, 40, clock(270), ev)
	}

	fmt.Println("step 6: a network glitch drops the heartbeats of 10 copies")
	r = &Registry{Leases: map[string]int{}}
	for _, c := range copies {
		alive[c] = true
		r.Register(c, 600)
	}
	beat(630, 720, func(c string, t int) bool { return t > 660 && c > "copy10" })
	got := 0
	for _, s := range r.Renews {
		if s > 660 {
			got++
		}
	}
	ev := r.Evict(720)
	fmt.Printf("  %s: renewals in last minute %d of 40 = %d%% < 85%%: self-preservation %v, evicted %v\n",
		clock(720), got, got*100/40, r.SelfPreservation, ev)
	alive["copy03"] = false
	ev = r.Evict(840)
	fmt.Printf("step 7: %s still preserving %v, evicted %v; copy03 stays listed\n", clock(840), r.SelfPreservation, ev)
	fmt.Println("step 8: client call with cache [copy03 copy07] -> tried", Call([]string{"copy03", "copy07"}, alive))
}
