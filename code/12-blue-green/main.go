// Command bluegreen replays the lesson: two full lanes, one pointer. A release goes
// to the idle lane, is proven there, and the pointer flips. Rollback is the same flip.
// Then the two ways it breaks: a shared schema change and a lane that drifted.
package main

import "fmt"

// Lane is one full environment.
type Lane struct {
	Name    string
	Version string
	MemGB   int
	Reads   string // the column of the shared table this version reads
	Broken  bool   // the version has a bug that only real load shows
}

// Env is the two lanes, the pointer and the shared database.
type Env struct {
	A, B *Lane
	Live *Lane           // the pointer: every agent query goes here
	DB   map[string]bool // columns of the shared table
}

// Idle is the lane the pointer does not name.
func (e *Env) Idle() *Lane {
	if e.Live == e.A {
		return e.B
	}
	return e.A
}

// Flip points every new query at the other lane in one step.
func (e *Env) Flip() { e.Live = e.Idle() }

// Serve answers one query on lane l under real load needing needGB of memory.
func (e *Env) Serve(l *Lane, needGB int) string {
	switch {
	case !e.DB[l.Reads]:
		return l.Name + " " + l.Version + ": error, column " + l.Reads + " missing"
	case l.MemGB < needGB:
		return fmt.Sprintf("%s %s: out of memory, %d GB needed, %d GB given", l.Name, l.Version, needGB, l.MemGB)
	case l.Broken:
		return l.Name + " " + l.Version + ": error on one kind of query"
	}
	return l.Name + " " + l.Version + ": ok"
}

// Drift lists the settings that differ between the lanes, other than the version.
func (e *Env) Drift() []string {
	var d []string
	if e.A.MemGB != e.B.MemGB {
		d = append(d, fmt.Sprintf("memory: lane A %d GB, lane B %d GB", e.A.MemGB, e.B.MemGB))
	}
	return d
}

func main() {
	const load = 24 // GB the proxy needs under real load
	e := &Env{
		A:  &Lane{"lane A", "3.37", 32, "dest", false},
		B:  &Lane{"lane B", "3.37", 32, "dest", false},
		DB: map[string]bool{"dest": true},
	}
	e.Live = e.A
	fmt.Println("live:", e.Serve(e.Live, load))

	idle := e.Idle()
	idle.Version = "3.38" // deploy to the idle lane: no agent is routed there
	fmt.Println("smoke test on", idle.Name+":", e.Serve(idle, load))
	e.Flip()
	fmt.Println("flip, live:", e.Serve(e.Live, load))
	e.Live.Broken = true // an hour later, under real load
	fmt.Println("an hour later:", e.Serve(e.Live, load))
	e.Flip()
	fmt.Println("flip back, live:", e.Serve(e.Live, load), "- one change, seconds")

	// The shared database: 3.38 renames dest to to_address.
	e.B = &Lane{"lane B", "3.38", 32, "to_address", false}
	e.Live = e.A
	e.DB = map[string]bool{"to_address": true}
	fmt.Println("\nrename before the flip, live:", e.Serve(e.Live, load))
	e.Flip()
	e.Live.Broken = true
	e.Flip()
	fmt.Println("3.38 has a bug, flip back, live:", e.Serve(e.Live, load))

	// The price: two full lanes, one serving.
	fmt.Printf("\ncost: %d GB paid for, %d GB serving\n", e.A.MemGB+e.B.MemGB, e.Live.MemGB)

	// Drift: lane A was raised to 32 GB in an incident, lane B stayed at 16 GB.
	e = &Env{A: &Lane{"lane A", "3.37", 32, "dest", false}, B: &Lane{"lane B", "3.38", 16, "dest", false}, DB: map[string]bool{"dest": true}}
	e.Live = e.A
	fmt.Println("drift check before the flip:", e.Drift())
	e.Flip()
	fmt.Println("flip anyway, live:", e.Serve(e.Live, load))
}
