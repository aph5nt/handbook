// Replays the space-based lesson: two processing units keep a seat map of
// 26 sections x 100 seats in memory, a data grid copies changes between them,
// and a data writer stores the changes in the database in batches. In the
// copy gap both units sell seat F14. One owner per section fixes it.
package main

import "fmt"

const batch = 500 // the data writer's batch size

type Change struct{ seat, buyer string }

type Unit struct {
	name  string
	seats map[string]string // seat -> buyer, "" when free
	inbox []Change          // changes sent by the other unit, not applied yet
	peer  *Unit             // the other unit
}

type Grid struct {
	units    []*Unit
	queue    []Change        // changes waiting for the data writer
	dbWrites int             // batch inserts into the database
	collided map[string]bool // seats sold to two buyers
	owned    bool            // one owner per section
}

func seatName(sec, n int) string { return fmt.Sprintf("%c%d", 'A'+sec, n) }

// loadSeats builds a unit's whole seat map in memory, once, before the sale.
func loadSeats() map[string]string {
	seats := map[string]string{}
	for sec := 0; sec < 26; sec++ {
		for i := 1; i <= 100; i++ {
			seats[seatName(sec, i)] = ""
		}
	}
	return seats
}

func newGrid(owned bool) *Grid {
	u1 := &Unit{name: "unit 1", seats: loadSeats()}
	u2 := &Unit{name: "unit 2", seats: loadSeats()}
	u1.peer, u2.peer = u2, u1
	return &Grid{units: []*Unit{u1, u2}, collided: map[string]bool{}, owned: owned}
}

// owner: sections A to M belong to unit 1, N to Z to unit 2.
func (g *Grid) owner(seat string) *Unit {
	if seat[0] <= 'M' {
		return g.units[0]
	}
	return g.units[1]
}

// request takes a buyer's request. With owners, it is forwarded first.
func (g *Grid) request(u *Unit, seat, buyer string) bool {
	if g.owned {
		u = g.owner(seat) // forward to the one writing owner
	}
	return g.sell(u, seat, buyer)
}

func (g *Grid) sell(u *Unit, seat, buyer string) bool {
	if u.seats[seat] != "" { // reads this unit's own memory
		return false // taken
	}
	u.seats[seat] = buyer                                    // memory only, no database call
	u.peer.inbox = append(u.peer.inbox, Change{seat, buyer}) // the grid copies it
	g.queue = append(g.queue, Change{seat, buyer})           // for the data writer
	return true
}

// deliver applies the copied changes, a moment after the sale.
func (g *Grid) deliver() {
	for _, u := range g.units {
		for _, c := range u.inbox {
			if b := u.seats[c.seat]; b != "" && b != c.buyer {
				g.collided[c.seat] = true // already sold to someone else
				continue
			}
			u.seats[c.seat] = c.buyer
		}
		u.inbox = nil
	}
}

// flush is the data writer: one batch insert per 500 changes (write-behind).
func (g *Grid) flush() {
	for len(g.queue) > 0 {
		n := min(batch, len(g.queue))
		g.queue = g.queue[n:]
		g.dbWrites++
	}
}

func sale(owned bool) {
	g := newGrid(owned)
	fmt.Printf("  loaded %d seats into each of %d units\n", len(g.units[0].seats), len(g.units))
	sold, i := 0, 0
	for sec := 0; sec < 26; sec++ { // the burst: buyers take every odd seat
		for n := 1; n <= 100; n += 2 {
			u := g.units[i%2] // the load balancer alternates units
			if g.request(u, seatName(sec, n), fmt.Sprintf("buyer %d", 100+i)) {
				sold++
			}
			if seatName(sec, n) == "F13" {
				fmt.Printf("  F13 sold by %s, copied to the other unit\n", u.name)
			}
			i++
		}
	}
	g.deliver()
	during := g.dbWrites // nothing has touched the database yet
	g.flush()
	fmt.Printf("  burst: %d sales, database writes while selling: %d, batch writes after: %d\n", sold, during, g.dbWrites)

	// the race: two buyers ask for F14 at two units before the copy arrives
	ok1 := g.request(g.units[0], "F14", "buyer 1")
	ok2 := g.request(g.units[1], "F14", "buyer 2")
	g.deliver()
	fmt.Printf("  F14: buyer 1 got it=%v, buyer 2 got it=%v, seats sold twice: %d\n", ok1, ok2, len(g.collided))
}

func main() {
	fmt.Println("without owners (any unit sells any seat):")
	sale(false)
	fmt.Println("with one owner per section (A to M unit 1, N to Z unit 2):")
	sale(true)
}
