// Cell-based architecture: the story of the lesson 12-cells.html.
// A platform runs three cells, each a full small copy with its own slice of
// customers. A router maps customer to cell. A bad deploy hurts one cell; a
// shared auth service or a down router can still hurt them all.
package main

import (
	"fmt"
	"sort"
)

const maxPerCell = 3 // a cell has a fixed maximum size

type Cell struct {
	ID      int
	Release string
	OwnAuth bool
}

type Platform struct {
	Route      map[string]int // the router: one row per customer
	Cells      map[int]*Cell
	SharedAuth bool // false when the shared auth service is down
	RouterUp   bool
}

// Serve routes a customer to its cell and nowhere else.
func (p *Platform) Serve(customer string) error {
	if !p.RouterUp {
		return fmt.Errorf("router down: no route to any cell")
	}
	c := p.Cells[p.Route[customer]]
	if !c.OwnAuth && !p.SharedAuth {
		return fmt.Errorf("cell %d: shared auth refused the token", c.ID)
	}
	if c.Release == "v2-bug" {
		return fmt.Errorf("cell %d: workers crash", c.ID)
	}
	return nil
}

// Deploy rolls a release one cell at a time and stops at the first cell
// whose error rate passes 1 percent.
func (p *Platform) Deploy(release string, order []int) {
	for _, id := range order {
		old := p.Cells[id].Release
		p.Cells[id].Release = release
		if r := p.ErrorRate(id); r > 0.01 {
			p.Cells[id].Release = old
			fmt.Printf("cell %d: error rate %.0f%% > 1%%: stop the roll, roll back\n", id, r*100)
			return
		}
	}
}

func (p *Platform) ErrorRate(id int) float64 {
	bad, n := 0, 0
	for cust, c := range p.Route {
		if c == id {
			n++
			if p.Serve(cust) != nil {
				bad++
			}
		}
	}
	return float64(bad) / float64(n)
}

// Place puts a new customer in the first cell with room, or a new cell.
func (p *Platform) Place(customer string) int {
	count := map[int]int{}
	for _, c := range p.Route {
		count[c]++
	}
	ids := []int{}
	for id := range p.Cells {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		if count[id] < maxPerCell {
			p.Route[customer] = id
			return id
		}
	}
	id := len(ids) + 1
	p.Cells[id] = &Cell{ID: id, Release: "v1", OwnAuth: true}
	p.Route[customer] = id
	return id
}

func (p *Platform) Failing() int {
	n := 0
	for cust := range p.Route {
		if p.Serve(cust) != nil {
			n++
		}
	}
	return n
}

func newPlatform(ownAuth bool) *Platform {
	p := &Platform{Route: map[string]int{}, Cells: map[int]*Cell{}, SharedAuth: true, RouterUp: true}
	for id := 1; id <= 3; id++ {
		p.Cells[id] = &Cell{ID: id, Release: "v1", OwnAuth: ownAuth}
	}
	names := []string{"Bolt", "Cora", "Dune", "Acme", "Eon", "Fig", "Gale", "Hux", "Iris"}
	for i, n := range names {
		p.Route[n] = i/3 + 1
	}
	return p
}

func main() {
	p := newPlatform(true)
	fmt.Printf("router: Acme -> cell %d\n", p.Route["Acme"])
	p.Cells[1].Release = "v2-bug"
	fmt.Printf("bad deploy on cell 1: %d of %d customers fail\n", p.Failing(), len(p.Route))
	p.Cells[1].Release = "v1"
	p.Deploy("v2-bug", []int{1, 2, 3})
	fmt.Printf("after the stopped roll: %d of %d fail, cell 2 runs %s\n", p.Failing(), len(p.Route), p.Cells[2].Release)
	fmt.Printf("all cells full at %d: new customer Jade goes to cell %d\n", maxPerCell, p.Place("Jade"))

	fmt.Println("== The shared piece")
	s := newPlatform(false)
	s.SharedAuth = false
	fmt.Printf("shared auth down: %d of %d customers fail\n", s.Failing(), len(s.Route))
	s.SharedAuth = true
	s.RouterUp = false
	fmt.Printf("router down: %d of %d customers fail\n", s.Failing(), len(s.Route))
}
