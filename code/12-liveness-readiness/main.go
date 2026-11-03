// Command livenessreadiness replays the lesson: Kubernetes probes three copies every
// 10 s. Readiness failing takes a copy out of the balancer. Liveness failing 3 times in
// a row restarts it. A liveness check that calls the database turns a 30 s database blip
// into a restart of every copy. Time moves in 10 s steps, so every run prints the same.
package main

import "fmt"

const (
	period           = 10 // seconds between probes
	failureThreshold = 3  // liveness failures in a row before a restart
	loadSeconds      = 60 // cache load after a start
)

// Copy is one running copy of the proxy.
type Copy struct {
	Name      string
	LoadedAt  int  // the time its cache is loaded
	Deadlock  bool // the handler no longer answers
	liveFails int
	Restarts  int
}

// World is what a probe can see at this moment.
type World struct {
	Now  int
	DBUp bool
}

// Liveness is the question "is the process itself alive?"
func Liveness(c *Copy, w World) bool { return !c.Deadlock }

// LivenessWrong also calls the database: the mistake.
func LivenessWrong(c *Copy, w World) bool { return !c.Deadlock && w.DBUp }

// Readiness is the question "can you take traffic right now?"
func Readiness(c *Copy, w World) bool {
	return !c.Deadlock && w.DBUp && w.Now >= c.LoadedAt
}

// Tick runs one probe round. It returns how many copies are in the balancer.
func Tick(copies []*Copy, w World, live func(*Copy, World) bool) int {
	ready := 0
	for _, c := range copies {
		if w.Now >= c.LoadedAt { // the startup probe holds the other checks back until loaded
			if live(c, w) {
				c.liveFails = 0
			} else {
				c.liveFails++
			}
			if c.liveFails >= failureThreshold {
				c.liveFails = 0
				c.Restarts++
				c.Deadlock = false
				c.LoadedAt = w.Now + loadSeconds // restarted: the cache loads again
				continue
			}
		}
		if Readiness(c, w) {
			ready++
		}
	}
	return ready
}

func fleet() []*Copy {
	return []*Copy{{Name: "copy 1"}, {Name: "copy 2"}, {Name: "copy 3"}}
}

// Blip runs a database outage from t=10 to t=40 and returns the outage length seen by agents.
func Blip(live func(*Copy, World) bool) (outage, restarts int) {
	copies := fleet()
	for t := 0; t <= 150; t += period {
		w := World{Now: t, DBUp: t < 10 || t >= 40}
		if Tick(copies, w, live) == 0 {
			outage += period
		}
	}
	for _, c := range copies {
		restarts += c.Restarts
	}
	return
}

func main() {
	o, r := Blip(Liveness)
	fmt.Printf("right: 30 s database blip: %d s with no copy ready, %d restarts\n", o, r)
	o, r = Blip(LivenessWrong)
	fmt.Printf("wrong: 30 s database blip: %d s with no copy ready, %d restarts\n", o, r)

	// A real deadlock: only that copy restarts, the other two keep serving.
	copies := fleet()
	copies[1].Deadlock = true
	lowest := 3
	for t := 0; t <= 100; t += period {
		if n := Tick(copies, World{Now: t, DBUp: true}, Liveness); n < lowest {
			lowest = n
		}
	}
	fmt.Printf("deadlock in copy 2: %d restart of copy 2, copy 1 %d, copy 3 %d, never fewer than %d ready\n",
		copies[1].Restarts, copies[0].Restarts, copies[2].Restarts, lowest)

	// A slow start with no startup probe: liveness judges a copy that is still loading.
	loading, kills := 0, 0
	for t := 0; t < 300; t += period {
		if t-loading >= loadSeconds {
			fmt.Println("no startup probe: ready")
			break
		}
		if t-loading == failureThreshold*period { // 3 failures while the cache loads
			kills++
			loading = t
		}
	}
	fmt.Printf("no startup probe: load takes %d s, liveness kills at %d s, %d kills in 5 minutes, never ready\n",
		loadSeconds, failureThreshold*period, kills)
}
