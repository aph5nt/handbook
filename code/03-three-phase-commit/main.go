// Three-phase commit: one phone order, a payment and a stock participant.
// It replays the lesson: a crash after pre-commit (both commit by timeout), a
// crash before it (both abort), a partition that splits the outcome, and the
// real fix, a coordinator replicated across three nodes.
package main

import "fmt"

// Participant is one service in the commit.
type Participant struct {
	Name  string
	State string // idle, voted, pre-committed, committed, aborted
}

// OnCanCommit locks the rows and votes yes.
func (p *Participant) OnCanCommit() bool {
	p.State = "voted"
	return true
}

// OnPreCommit records that everyone voted yes.
func (p *Participant) OnPreCommit() { p.State = "pre-committed" }

// OnDoCommit makes the change final.
func (p *Participant) OnDoCommit() { p.State = "committed" }

// OnTimeout decides alone, from what this participant knows.
func (p *Participant) OnTimeout() {
	switch p.State {
	case "pre-committed":
		p.State = "committed" // everyone voted yes
	case "voted":
		p.State = "aborted" // it never heard the result
	}
}

// Scenario says where the coordinator stops, and who pre-commit can reach.
type Scenario struct {
	CrashBeforePreCommit bool
	CrashBeforeDoCommit  bool
	PreCommitReaches     map[string]bool
}

// Run3PC is the coordinator. After a crash, every participant times out.
func Run3PC(parts []*Participant, sc Scenario) {
	for _, p := range parts {
		if !p.OnCanCommit() {
			return
		}
	}
	if !sc.CrashBeforePreCommit {
		for _, p := range parts {
			if sc.PreCommitReaches[p.Name] {
				p.OnPreCommit()
			}
		}
		if !sc.CrashBeforeDoCommit {
			for _, p := range parts {
				p.OnDoCommit()
			}
			return
		}
	}
	for _, p := range parts {
		p.OnTimeout() // the coordinator is gone
	}
}

// Group is a coordinator replicated on three nodes.
type Group struct {
	alive    int
	decision string
}

// Decide writes the outcome only if a majority of the group stores it.
func (g *Group) Decide(outcome string) bool {
	if 2*g.alive <= 3 {
		return false
	}
	g.decision = outcome
	return true
}

// RunGroup: no timeout decides. Participants learn the group's decision.
func RunGroup(parts []*Participant, g *Group) {
	for _, p := range parts {
		p.OnCanCommit()
	}
	if g.Decide("commit") {
		for _, p := range parts {
			p.OnDoCommit()
		}
	}
}

func show(label string, parts []*Participant) {
	fmt.Printf("%s: payment %s, stock %s\n", label, parts[0].State, parts[1].State)
}

func order() []*Participant {
	return []*Participant{{"payment", "idle"}, {"stock", "idle"}}
}

func main() {
	all := map[string]bool{"payment": true, "stock": true}
	fmt.Println("== Coordinator crash")
	parts := order()
	Run3PC(parts, Scenario{PreCommitReaches: all})
	show("no failure", parts)
	parts = order()
	Run3PC(parts, Scenario{CrashBeforeDoCommit: true, PreCommitReaches: all})
	show("crash after pre-commit", parts)
	parts = order()
	Run3PC(parts, Scenario{CrashBeforePreCommit: true})
	show("crash before pre-commit", parts)

	fmt.Println("== Partition")
	parts = order()
	Run3PC(parts, Scenario{CrashBeforeDoCommit: true, PreCommitReaches: map[string]bool{"payment": true}})
	show("pre-commit reached only payment", parts)

	fmt.Println("== Fix: coordinator group")
	parts = order()
	RunGroup(parts, &Group{alive: 2})
	show("one of three coordinator replicas lost", parts)
}
