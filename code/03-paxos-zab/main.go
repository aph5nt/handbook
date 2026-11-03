// Paxos: three acceptors agree who leads. It replays the lesson: acceptor 2
// already accepted "node 2" in ballot 4, so a proposer that wants "node 1"
// must carry "node 2" forward. Then two proposers duel with rising ballots
// until one backs off and the other gets its value chosen.
package main

import "fmt"

// Acceptor remembers the highest ballot it promised and what it accepted.
type Acceptor struct {
	name      string
	up        bool
	promised  int
	accBallot int
	accValue  string
}

// Promise is an acceptor's reply to prepare.
type Promise struct {
	ok        bool
	accBallot int
	accValue  string
}

// Prepare: promise to ignore smaller ballots, and report what was accepted.
func (a *Acceptor) Prepare(b int) Promise {
	if !a.up || b <= a.promised {
		return Promise{ok: false}
	}
	a.promised = b
	return Promise{ok: true, accBallot: a.accBallot, accValue: a.accValue}
}

// Accept: take (b, v) unless a higher ballot was promised since.
func (a *Acceptor) Accept(b int, v string) bool {
	if !a.up || b < a.promised {
		return false // promised a higher ballot
	}
	a.promised, a.accBallot, a.accValue = b, b, v
	return true
}

// Phase1 sends prepare to the acceptors it reaches and picks the value to
// propose. It returns false without a majority of all n acceptors.
func Phase1(acc []*Acceptor, n, b int, want string) (string, bool) {
	promises, best, v := 0, 0, want
	for _, a := range acc {
		p := a.Prepare(b)
		if !p.ok {
			continue
		}
		promises++
		if p.accValue != "" && p.accBallot > best {
			best, v = p.accBallot, p.accValue // carry the value forward
		}
	}
	return v, promises > n/2
}

// Phase2 sends accept. The value is chosen if a majority of n accepts.
func Phase2(acc []*Acceptor, n, b int, v string) bool {
	oks := 0
	for _, a := range acc {
		if a.Accept(b, v) {
			oks++
		}
	}
	return oks > n/2
}

func main() {
	fmt.Println("== Carry the value forward")
	a1 := &Acceptor{name: "acceptor 1", up: true}
	a2 := &Acceptor{name: "acceptor 2", up: true, promised: 4, accBallot: 4, accValue: "leader = node 2"}
	a3 := &Acceptor{name: "acceptor 3", up: false}
	acc := []*Acceptor{a1, a2, a3}
	v, ok := Phase1(acc, 3, 5, "leader = node 1")
	fmt.Printf("prepare ballot 5: majority of promises=%v, must propose %q (wanted \"leader = node 1\")\n", ok, v)
	fmt.Printf("accept ballot 5: chosen=%v\n", Phase2(acc, 3, 5, v))

	fmt.Println("== Duelling proposers")
	d := []*Acceptor{{name: "a1", up: true}, {name: "a2", up: true}, {name: "a3", up: true}}
	p1, p2 := []*Acceptor{d[0], d[1]}, []*Acceptor{d[1], d[2]} // who each one reaches first
	b1, b2 := 5, 6
	for round := 0; round < 3; round++ {
		v1, _ := Phase1(p1, 3, b1, "worker 1")
		_, _ = Phase1(p2, 3, b2, "worker 2") // a higher ballot steals acceptor 2
		chosen := Phase2(p1, 3, b1, v1)
		fmt.Printf("round %d: proposer 1 accept ballot %d chosen=%v (acceptor 2 promised %d)\n", round+1, b1, chosen, d[1].promised)
		b1, b2 = b2+1, b2+2
	}
	fmt.Println("fix: proposer 2 backs off, proposer 1 leads alone with ballot 11")
	v, _ = Phase1(d, 3, 11, "worker 1")
	fmt.Printf("ballot 11: %q chosen=%v\n", v, Phase2(d, 3, 11, v))
}
