// Replays the serverless lesson with a small model of a function platform.
// Bursty thumbnails fit: copies start on demand and stop when idle. A chain
// indexer does not: calls hit the time limit, copies run the database out of
// connections, and per-call billing costs more at steady load.
//
// Model values (assumptions of this model, not measurements):
// cold start 800 ms, thumbnail 600 ms, idle stop 10 minutes, time limit
// 15 minutes (the AWS Lambda limit), 5 connections per copy, database max
// 100, price 1.0 per busy second for a function, 0.4 per second for a
// container that runs all the time.
package main

import "fmt"

const (
	coldMs      = 800
	thumbMs     = 600
	idleStopMin = 10
	limitMin    = 15
	connsCopy   = 5
	dbMax       = 100
	priceFn     = 1.0 // per busy second
	priceBox    = 0.4 // per second, running or not
)

type Copy struct {
	busy     bool
	idleFrom int // minute the copy became idle
}

type Platform struct {
	copies  []*Copy
	busyMs  int // billed time
	calls   int
	dbConns int
}

// invoke runs one event on an idle warm copy, or starts a new copy.
func (p *Platform) invoke(workMs int) (cold bool, ms int) {
	var c *Copy
	for _, x := range p.copies {
		if !x.busy { // an idle warm copy
			c = x
			break
		}
	}
	if c == nil { // none idle: start a fresh copy
		c = &Copy{}
		p.copies = append(p.copies, c)
		cold, ms = true, coldMs
	}
	c.busy = true
	ms += workMs
	p.busyMs += ms // billed per call, for the busy time
	p.calls++
	return cold, ms
}

func (p *Platform) finish(now int) {
	for _, c := range p.copies {
		c.busy, c.idleFrom = false, now
	}
}

// reap stops copies idle for idleStopMin minutes: scale to zero.
func (p *Platform) reap(now int) {
	var keep []*Copy
	for _, c := range p.copies {
		if c.busy || now-c.idleFrom < idleStopMin {
			keep = append(keep, c)
		}
	}
	p.copies = keep
}

// runLong runs work with the platform's time limit.
func runLong(needMin int) (ranMin int, killed bool) {
	if needMin > limitMin {
		return limitMin, true // the call is cut off
	}
	return needMin, false
}

// connect opens a new copy's own database connections.
func (p *Platform) connect() bool {
	if p.dbConns+connsCopy > dbMax {
		return false // refused
	}
	p.dbConns += connsCopy
	return true
}

func main() {
	fmt.Println("tab 1: bursty work fits")
	p := &Platform{}
	fmt.Printf("  night: copies %d, calls %d, billed %.1f\n", len(p.copies), p.calls, float64(p.busyMs)/1000*priceFn)
	cold, ms := p.invoke(thumbMs)
	fmt.Printf("  upload 1: cold=%v, %d ms (%d cold start + %d resize), copies %d\n", cold, ms, coldMs, thumbMs, len(p.copies))
	p.finish(0)
	colds := 0
	for i := 0; i < 30; i++ { // a school uploads 30 photos at once
		if c, _ := p.invoke(thumbMs); c {
			colds++
		}
	}
	fmt.Printf("  burst of 30: %d warm copy reused, %d cold starts, copies %d\n", 30-colds, colds, len(p.copies))
	p.finish(1)
	p.reap(1 + idleStopMin)
	fmt.Printf("  after %d idle minutes: copies %d\n", idleStopMin, len(p.copies))
	fmt.Printf("  day: %d calls, %.1f busy seconds, cost %.1f vs container all day %.1f\n",
		p.calls, float64(p.busyMs)/1000, float64(p.busyMs)/1000*priceFn, 86400*priceBox)

	fmt.Println("tab 2: long, steady work does not")
	ran, killed := runLong(25)
	fmt.Printf("  a range needs 25 minutes: ran %d, killed=%v, %d percent done\n", ran, killed, ran*100/25)
	q := &Platform{}
	ok, refused := 0, 0
	for i := 0; i < 40; i++ { // catch up with 40 copies at once
		if q.connect() {
			ok++
		} else {
			refused++
		}
	}
	fmt.Printf("  40 copies x %d connections = %d wanted, max %d: %d connected, %d refused\n", connsCopy, 40*connsCopy, dbMax, ok, refused)
	fn, box := 86400*priceFn, 86400*priceBox
	fmt.Printf("  busy all day: function %.0f vs container %.0f (%.1f times more)\n", fn, box, fn/box)
	one := &Platform{}
	one.connect() // one container, one set of connections, kept all day
	fmt.Printf("  fix: one container, %d connections, no time limit\n", one.dbConns)
}
