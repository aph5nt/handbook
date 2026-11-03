// Event sourcing: write the event, apply it after the write, snapshot every 1,000 events,
// and rebuild on another node from the snapshot plus the events after it.
package main

import "fmt"

type Event struct {
	Seq  int
	Kind string
	N    int
}

// Store is the journal and the snapshot store. It lives in the database, not on a node.
type Store struct {
	journal  []Event
	snapshot *State
}

func (s *Store) Persist(kind string, n int) Event {
	e := Event{len(s.journal) + 1, kind, n}
	s.journal = append(s.journal, e)
	return e
}

type State struct {
	Seq      int
	Online   bool
	Interval int
}

// apply is the only place that changes state. Recovery and live writes share it.
func (st *State) apply(e Event) {
	switch e.Kind {
	case "Connected":
		st.Online = true
	case "Disconnected":
		st.Online = false
	case "ConfigChanged":
		st.Interval = e.N
	}
	st.Seq = e.Seq
}

// recover loads the snapshot, then replays only the events after it.
func recoverState(s *Store) (State, int) {
	st := State{Interval: 60}
	if s.snapshot != nil {
		st = *s.snapshot
	}
	replayed := 0
	for _, e := range s.journal[st.Seq:] {
		st.apply(e)
		replayed++
	}
	return st, replayed
}

type Cmd struct {
	Kind string
	N    int
}

// device is the actor: one goroutine, one mailbox, the state in memory.
func device(store *Store, mailbox chan Cmd, done chan State, every int) {
	st := State{Interval: 60}
	for c := range mailbox {
		if c.Kind == "ConfigChanged" && c.N == st.Interval {
			continue // the command decides: nothing changes, no event
		}
		e := store.Persist(c.Kind, c.N) // write first
		st.apply(e)                     // apply only after the write
		if every > 0 && e.Seq%every == 0 {
			snap := st
			store.snapshot = &snap
			fmt.Printf("  snapshot at event %d: interval %d s\n", e.Seq, st.Interval)
		}
	}
	done <- st
}

func main() {
	fmt.Println("== decisions only, node 1")
	store := &Store{}
	mb := make(chan Cmd, 1200)
	done := make(chan State)
	go device(store, mb, done, 1000)
	mb <- Cmd{"Connected", 0}
	mb <- Cmd{Kind: "ConfigChanged", N: 30}
	for i := 0; i < 998; i++ { // 998 more over the weeks
		mb <- Cmd{Kind: "ConfigChanged", N: 31 + i%2}
	}
	mb <- Cmd{Kind: "ConfigChanged", N: 60}
	mb <- Cmd{"Disconnected", 0}
	close(mb)
	onNode1 := <-done
	fmt.Printf("  node 1: %+v, journal %d events\n", onNode1, len(store.journal))

	fmt.Println("== node 1 dies, the state in memory is gone. Node 2 rebuilds")
	st, n := recoverState(store)
	fmt.Printf("  node 2: loaded snapshot at event %d, replayed %d events: %+v\n", store.snapshot.Seq, n, st)
	fmt.Printf("  same state as node 1: %v\n", st == onNode1)
	noSnap := &Store{journal: store.journal}
	_, all := recoverState(noSnap)
	fmt.Printf("  with no snapshot it would replay %d events\n", all)

	fmt.Println("== every reading as an event")
	perDay := 24 * 60 * 60
	readings := &Store{}
	for sec := 0; sec < perDay; sec++ {
		readings.Persist("ReadingStored", 215) // the trap: a reading as an event
	}
	_, replay := recoverState(readings)
	fmt.Printf("  one device: %d events a day, replay with no snapshot: %d events\n", len(readings.journal), replay)
	fmt.Printf("  10,000 devices: %d events a day\n", perDay*10000)
	fmt.Printf("  a snapshot every 1,000 readings: every %.1f minutes per device\n", 1000.0/60)
	fmt.Printf("  worst case replay after a crash: 999 readings, and the journal keeps growing\n")

	fmt.Println("== the fix: decisions in the journal, readings to the warehouse in batches")
	var warehouse []int // readings flow through in batches, never into the journal
	for sec := 0; sec < perDay; sec++ {
		warehouse = append(warehouse, 215)
	}
	fmt.Printf("  warehouse batch: %d readings, journal untouched\n", len(warehouse))
	small := &Store{}
	for _, c := range []Cmd{{"Connected", 0}, {"ConfigChanged", 30}, {"Disconnected", 0}} {
		small.Persist(c.Kind, c.N)
	}
	st, n = recoverState(small)
	fmt.Printf("  node 2: replayed %d events: %+v\n", n, st)
}
