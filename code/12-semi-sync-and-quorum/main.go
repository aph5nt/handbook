// Semi-sync and quorum writes: the story of the lesson
// 12-semi-sync-and-quorum.html, replayed. Semi-sync waits for one replica and
// quietly falls back to async on a timeout. A quorum store needs W + R > N.
package main

import "fmt"

const timeoutMs = 3000 // how long the primary waits for one confirmation

// Replica confirms a copy after AckMs.
type Replica struct {
	Name    string
	Balance int
	AckMs   int
}

// Primary holds the newest balance and the mode it runs in.
type Primary struct {
	Balance  int
	Async    bool // true after the silent fallback
	Replicas []*Replica
}

// SemiWrite answers after the first confirmation, or after the timeout with no
// confirmation at all. Returns how long the client waited and if it is safe.
func (p *Primary) SemiWrite(balance int) (waitMs int, confirmed bool) {
	p.Balance = balance
	fastest := p.Replicas[0].AckMs
	for _, r := range p.Replicas {
		fastest = min(fastest, r.AckMs)
	}
	if fastest > timeoutMs {
		p.Async = true // no error, no sign to the client
		return timeoutMs, false
	}
	for _, r := range p.Replicas {
		r.Balance = balance // the slower one arrives moments later
	}
	return fastest, true
}

// Promote picks replica 1 after the primary dies. The second result says if a
// write the client saw as ok is gone.
func (p *Primary) Promote() (int, bool) {
	return p.Replicas[0].Balance, p.Replicas[0].Balance != p.Balance
}

// Copy is one equal copy of a key in the quorum store.
type Copy struct {
	Version int
	Up      bool
}

// Write sends to the listed copies and succeeds on W confirmations.
func Write(cs []*Copy, version, w int, reach ...int) bool {
	acks := 0
	for _, k := range reach {
		if cs[k].Up {
			cs[k].Version = version
			acks++
		}
	}
	return acks >= w
}

// Read asks the listed copies and returns the newest version it sees.
func Read(cs []*Copy, ask ...int) int {
	newest := 0
	for _, k := range ask {
		newest = max(newest, cs[k].Version)
	}
	return newest
}

// Overlap counts write sets and read sets that share a copy, out of all pairs.
func Overlap(n, w, r int) (shared, total int) {
	for ws := 0; ws < 1<<n; ws++ {
		for rs := 0; rs < 1<<n; rs++ {
			if bits(ws) == w && bits(rs) == r {
				total++
				if ws&rs != 0 {
					shared++
				}
			}
		}
	}
	return shared, total
}

func bits(x int) int {
	c := 0
	for ; x > 0; x >>= 1 {
		c += x & 1
	}
	return c
}

func main() {
	fmt.Println("== semi: semi-sync and the silent fallback")
	p := &Primary{Balance: 100, Replicas: []*Replica{{"replica 1", 100, 8}, {"replica 2", 100, 30}}}
	ms, ok := p.SemiWrite(90)
	fmt.Printf("1 pay 10, copies sent: acks in %d and %d ms\n", p.Replicas[0].AckMs, p.Replicas[1].AckMs)
	fmt.Printf("2 first ack: client told ok after %d ms, confirmed %v\n", ms, ok)
	bal, lost := p.Promote()
	fmt.Printf("3 primary dies: replica 1 promoted with %d, write lost %v\n", bal, lost)
	p.Replicas[0].AckMs, p.Replicas[1].AckMs = 8000, 9000
	fmt.Printf("4 replicas slow: acks would take %d and %d ms, timeout %d ms\n", 8000, 9000, timeoutMs)
	ms, ok = p.SemiWrite(80)
	fmt.Printf("5 timeout: client told ok after %d ms, confirmed %v, async %v, replicas hold %d\n", ms, ok, p.Async, p.Replicas[0].Balance)
	bal, lost = p.Promote()
	fmt.Printf("6 primary dies: replica 1 promoted with %d, write lost %v\n", bal, lost)

	fmt.Println("== quorum: W + R > N, N = 3")
	cs := []*Copy{{1, true}, {1, true}, {1, false}}
	fmt.Println("1 all copies at v1 (copy 3 about to go down)")
	fmt.Println("2 write v2, W 2, copy 3 down: ok =", Write(cs, 2, 2, 0, 1, 2), " copies", cs[0].Version, cs[1].Version, cs[2].Version)
	cs[2].Up = true
	fmt.Printf("3 copy 3 back at v1, R 2 asks copies 2 and 3: newest v%d\n", Read(cs, 1, 2))
	s, t := Overlap(3, 2, 2)
	fmt.Printf("  W 2, R 2: %d of %d write-read pairs share a copy\n", s, t)
	Write(cs, 3, 1, 0)
	fmt.Printf("4 W 1, R 1: v3 on copy 1 only, read copy 3: v%d\n", Read(cs, 2))
	s, t = Overlap(3, 1, 1)
	fmt.Printf("  W 1, R 1: %d of %d pairs share a copy, %d can read old\n", s, t, t-s)
	fmt.Println("5 majority of 3 is", 3/2+1, "- write v3 on copies 1 and 2: ok =", Write(cs, 3, 2, 0, 1))
	s, t = Overlap(3, 2, 2)
	fmt.Printf("  two majority writes: %d of %d pairs share a copy\n", s, t)
}
