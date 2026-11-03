// Two-phase commit versus saga: one phone order across payment, stock and
// shipping. Shipping refuses the address. The program replays both protocols,
// the coordinator crash that blocks 2PC, and the orchestrator crash a saga
// survives.
package main

import "fmt"

// Participant is one service in 2PC, with its own database.
type Participant struct {
	name   string
	canDo  bool
	state  string // idle, locked, committed, rolled back, refused
	locked bool
}

// Prepare writes the change to the service's log and locks the rows.
func (p *Participant) Prepare() bool {
	if !p.canDo {
		p.state = "refused"
		return false
	}
	p.state, p.locked = "locked", true // the rows wait for the decision
	return true
}

// Finish applies the coordinator's decision and frees the locks.
func (p *Participant) Finish(decision string) {
	if p.state == "refused" || p.state == "idle" {
		return
	}
	p.locked = false
	if decision == "commit" {
		p.state = "committed"
	} else {
		p.state = "rolled back"
	}
}

// Coordinator drives one vote. log is its durable decision record.
type Coordinator struct {
	parts []*Participant
	log   string
}

// Run is 2PC. crashAfter > 0 stops the coordinator after that many votes.
func (c *Coordinator) Run(crashAfter int) string {
	decision := "commit"
	for k, p := range c.parts {
		if crashAfter > 0 && k == crashAfter {
			return "crashed" // no decision was written: everyone waits
		}
		if !p.Prepare() {
			decision = "abort" // one no is enough
		}
	}
	c.log = decision // durable before anyone hears the result
	for _, p := range c.parts {
		p.Finish(decision)
	}
	return decision
}

// Recover runs when the coordinator restarts. No logged decision means abort.
func (c *Coordinator) Recover() string {
	decision := c.log
	if decision == "" {
		decision = "abort"
		c.log = decision
	}
	for _, p := range c.parts {
		p.Finish(decision)
	}
	return decision
}

// Step is one local transaction of a saga, with its undo.
type Step struct {
	name, undo string
	ok         bool
}

// Saga is the orchestrator: a list of steps and a durable log of progress.
type Saga struct {
	steps []Step
	log   map[string]string // step name -> started or done
	calls map[string]int    // how often each service did the work
}

// do runs one step. The service keys the work by order id, so a repeat is safe.
func (s *Saga) do(st Step) bool {
	s.calls[st.name]++
	if st.ok {
		fmt.Printf("  %s commits at once\n", st.name)
	}
	return st.ok
}

// Run walks the steps. On a refusal it compensates the done steps backwards.
func (s *Saga) Run() string {
	var done []Step
	for _, st := range s.steps {
		s.log[st.name] = "started"
		if !s.do(st) {
			fmt.Printf("  %s refused\n", st.name)
			for k := len(done) - 1; k >= 0; k-- {
				fmt.Printf("  compensate: %s\n", done[k].undo)
			}
			return "cancelled"
		}
		s.log[st.name] = "done"
		done = append(done, st)
	}
	return "done"
}

// Resume runs after an orchestrator crash: it repeats what is not done.
func (s *Saga) Resume() string {
	for _, st := range s.steps {
		if s.log[st.name] == "done" {
			continue // already committed, never run twice
		}
		s.log[st.name] = "started"
		s.do(st)
		s.log[st.name] = "done"
	}
	return "done"
}

func show(parts []*Participant) string {
	out := ""
	for _, p := range parts {
		out += fmt.Sprintf("%s=%s ", p.name, p.state)
	}
	return out
}

func newParts() []*Participant {
	return []*Participant{{"payment", true, "idle", false}, {"stock", true, "idle", false}, {"shipping", false, "idle", false}}
}

func main() {
	fmt.Println("== 2PC: shipping votes no")
	c := &Coordinator{parts: newParts()}
	fmt.Println("decision:", c.Run(0), "|", show(c.parts))

	fmt.Println("== 2PC: coordinator crashes after two votes")
	c = &Coordinator{parts: newParts()}
	fmt.Println("result:", c.Run(2), "|", show(c.parts), "(locks held, nobody can decide)")
	fmt.Println("restart, log empty, decision:", c.Recover(), "|", show(c.parts))

	fmt.Println("== Saga: shipping refuses")
	s := &Saga{log: map[string]string{}, calls: map[string]int{}, steps: []Step{
		{"charge", "refund", true}, {"reserve", "release", true}, {"book", "", false}}}
	fmt.Println("order:", s.Run())

	fmt.Println("== Saga: orchestrator crashes after reserve, book was started")
	s = &Saga{calls: map[string]int{"charge": 1, "reserve": 1},
		log:   map[string]string{"charge": "done", "reserve": "done", "book": "started"},
		steps: []Step{{"charge", "refund", true}, {"reserve", "release", true}, {"book", "", true}}}
	fmt.Println("resume:", s.Resume(), "| charge ran", s.calls["charge"], "time, book ran", s.calls["book"], "time")
}
