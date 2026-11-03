// Virtual actors (Orleans grains). A grain exists by its key. The directory activates it
// on the first call and an idle timer deactivates it. A turn is one call, including its
// awaits, unless a method is reentrant. Time is scaled: 300 ms stands for the 30 s call
// timeout, 40 ms for the idle time.
package main

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

const callTimeout = 300 * time.Millisecond

var errTimeout = errors.New("call timed out")
var t0 time.Time

func log(f string, a ...any) {
	fmt.Printf("%4d ms  "+f+"\n", append([]any{time.Since(t0).Milliseconds()}, a...)...)
}

type Reply struct {
	Val int
	Err error
}

// Call is one message in a grain's mailbox.
type Call struct {
	Name      string
	Reentrant bool
	Fn        func(g *Grain) (int, error)
	Reply     chan Reply
}

type Grain struct {
	key     string
	silo    int
	score   int
	rt      *Runtime
	mailbox chan Call
	pending []Call // calls taken during an await that must wait their turn
}

// Runtime: the directory of activations, the storage, and the placement.
type Runtime struct {
	mu      sync.Mutex
	active  map[string]*Grain
	storage map[string]int
	silos   []int
	idle    time.Duration
}

func newRuntime(idle time.Duration, silos ...int) *Runtime {
	return &Runtime{active: map[string]*Grain{}, storage: map[string]int{}, silos: silos, idle: idle}
}

// send finds the activation, or creates it, while holding the directory lock.
func (rt *Runtime) send(key string, c Call) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	g, ok := rt.active[key]
	if !ok {
		silo := rt.silos[0]
		rt.silos = rt.silos[1:]
		g = &Grain{key: key, silo: silo, score: rt.storage[key], rt: rt, mailbox: make(chan Call, 16)}
		rt.active[key] = g
		log("runtime: activate %s on silo %d, load score %d", key, silo, g.score)
		go g.run()
	}
	g.mailbox <- c
}

// Ask is a call from outside any grain: it waits with a timeout.
func (rt *Runtime) Ask(key string, c Call) (int, error) {
	c.Reply = make(chan Reply, 1)
	rt.send(key, c)
	select {
	case r := <-c.Reply:
		return r.Val, r.Err
	case <-time.After(callTimeout):
		return 0, errTimeout
	}
}

// run is the grain's goroutine: one turn at a time, deactivate when idle.
func (g *Grain) run() {
	for {
		if len(g.pending) > 0 {
			c := g.pending[0]
			g.pending = g.pending[1:]
			g.turn(c)
			continue
		}
		var idle <-chan time.Time
		if g.rt.idle > 0 {
			idle = time.After(g.rt.idle)
		}
		select {
		case c := <-g.mailbox:
			g.turn(c)
		case <-idle:
			g.rt.mu.Lock()
			if len(g.mailbox) > 0 { // a call slipped in: stay active
				g.rt.mu.Unlock()
				continue
			}
			g.rt.storage[g.key] = g.score
			delete(g.rt.active, g.key)
			log("runtime: %s idle, save score %d, deactivate on silo %d", g.key, g.score, g.silo)
			g.rt.mu.Unlock()
			return
		}
	}
}

func (g *Grain) turn(c Call) {
	v, err := c.Fn(g)
	c.Reply <- Reply{v, err}
}

// Await is a call from inside a turn. A plain grain takes nothing while it waits. Only
// calls marked reentrant may run at the await point, unless all is set.
func (g *Grain) Await(key string, c Call, all bool) (int, error) {
	c.Reply = make(chan Reply, 1)
	g.rt.send(key, c)
	timer := time.After(callTimeout)
	for {
		select {
		case r := <-c.Reply:
			return r.Val, r.Err
		case <-timer:
			return 0, errTimeout
		case in := <-g.mailbox:
			if in.Reentrant || all {
				g.turn(in) // runs now, in the middle of this turn
			} else {
				g.pending = append(g.pending, in) // waits its turn
			}
		}
	}
}

func getRating(reentrant bool) Call {
	return Call{Name: "GetRating", Reentrant: reentrant, Fn: func(g *Grain) (int, error) {
		log("%s: GetRating runs", g.key)
		return 1500, nil
	}}
}

// addPlayer needs the rating. With withRating the caller already passed it.
func addPlayer(withRating, reentrant bool) Call {
	return Call{Name: "AddPlayer", Fn: func(m *Grain) (int, error) {
		time.Sleep(10 * time.Millisecond)
		rating := 1500
		if !withRating {
			var err error
			log("%s: in AddPlayer, GetRating, await", m.key)
			rating, err = m.Await("player/7", getRating(reentrant), false)
			if err != nil {
				log("%s: GetRating failed: %v", m.key, err)
				return 0, err
			}
		}
		log("%s: added player 7, rating %d", m.key, rating)
		return rating, nil
	}}
}

func joinMatch(c Call) Call {
	return Call{Name: "JoinMatch", Fn: func(p *Grain) (int, error) {
		time.Sleep(10 * time.Millisecond)
		log("%s: in JoinMatch, AddPlayer, await", p.key)
		_, err := p.Await("match/9", c, false)
		if err != nil {
			log("%s: AddPlayer failed: %v, JoinMatch ends", p.key, err)
			return 0, err
		}
		log("%s: JoinMatch done", p.key)
		return 0, nil
	}}
}

func section(s string) {
	fmt.Println("\n== " + s)
	t0 = time.Now()
}

func main() {
	section("activation: exists by key, activates on the first call")
	rt := newRuntime(40*time.Millisecond, 2, 1)
	log("GetGrain player/7: active activations %d", len(rt.active))
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		rt.Ask("player/7", Call{Name: "JoinLobby", Fn: func(g *Grain) (int, error) {
			log("player/7: JoinLobby starts, awaits a store for 20 ms")
			time.Sleep(20 * time.Millisecond)
			log("player/7: JoinLobby ends")
			return 0, nil
		}})
	}()
	time.Sleep(5 * time.Millisecond)
	go func() {
		defer wg.Done()
		rt.Ask("player/7", Call{Name: "AddScore", Fn: func(g *Grain) (int, error) {
			g.score += 120
			log("player/7: AddScore ran after JoinLobby, score %d", g.score)
			return g.score, nil
		}})
	}()
	wg.Wait()
	time.Sleep(60 * time.Millisecond) // quiet: the idle time passes
	s, _ := rt.Ask("player/7", Call{Name: "GetStats", Fn: func(g *Grain) (int, error) { return g.score, nil }})
	log("game server: GetStats -> score %d", s)
	_, err := rt.Ask("player/7", Call{Name: "GetStats", Fn: func(g *Grain) (int, error) { return 0, errors.New("stats failed") }})
	log("game server: no supervisor, the exception is mine: %v", err)
	time.Sleep(60 * time.Millisecond) // the idle time passes, the grain goes

	section("the cycle: player -> match -> player")
	rt = newRuntime(0, 2, 1)
	_, err = rt.Ask("player/7", joinMatch(addPlayer(false, false)))
	log("game server: JoinMatch returned: %v", err)
	time.Sleep(60 * time.Millisecond)

	section("fix 1: send what the callee needs, no cycle")
	rt = newRuntime(0, 2, 1)
	_, err = rt.Ask("player/7", joinMatch(addPlayer(true, false)))
	log("game server: JoinMatch returned: %v", err)

	section("fix 2: GetRating is reentrant")
	rt = newRuntime(0, 2, 1)
	_, err = rt.Ask("player/7", joinMatch(addPlayer(false, true)))
	log("game server: JoinMatch returned: %v", err)

	section("the cost: a call runs at the await and changes the state")
	rt = newRuntime(0, 2, 1)
	done := make(chan struct{})
	go func() {
		rt.Ask("player/7", Call{Name: "JoinMatch", Fn: func(p *Grain) (int, error) {
			p.score = 100
			before := p.score // read before the await
			log("player/7: read score %d, then await", before)
			p.Await("match/9", Call{Name: "Slow", Fn: func(m *Grain) (int, error) {
				time.Sleep(30 * time.Millisecond)
				return 0, nil
			}}, true) // the whole grain is reentrant
			p.score = before + 10 // the bonus, computed on the stale read
			log("player/7: score after the await %d, the +20 is lost", p.score)
			return 0, nil
		}})
		close(done)
	}()
	time.Sleep(10 * time.Millisecond)
	rt.Ask("player/7", Call{Name: "AddScore", Fn: func(g *Grain) (int, error) {
		g.score += 20
		log("player/7: AddScore ran inside JoinMatch, score %d", g.score)
		return g.score, nil
	}})
	<-done
}
