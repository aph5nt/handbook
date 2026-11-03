// Command cells replays the cells lesson: a thin router over three cells, a
// deploy wave that stops at the first bad cell, a poison request that stays
// in one cell, growth by adding a cell, and a bad router table.
package main

import "fmt"

type Cell struct {
	Name      string
	Version   int
	ErrorRate float64
	Restarts  int
}

type Router struct{ Table map[byte]int } // first letter -> cell index

func (r *Router) Route(customer string) (int, error) {
	i, ok := r.Table[customer[0]]
	if !ok {
		return 0, fmt.Errorf("no cell for %s", customer)
	}
	return i, nil
}

func Deploy(cells []*Cell, version int, errorsOf map[string]float64) (done []string, stoppedAt string) {
	for _, c := range cells {
		old := c.Version
		c.Version, c.ErrorRate = version, errorsOf[c.Name]
		if c.ErrorRate > 0.01 {
			c.Version = old
			return done, c.Name
		}
		done = append(done, c.Name)
	}
	return done, ""
}

func Handle(c *Cell, poison bool) string {
	if poison {
		c.Restarts++
		return c.Name + " crashed and restarted"
	}
	return c.Name + " ok"
}

// Table builds the router's lookup: letters from..to go to one cell.
func Table(t map[byte]int, from, to byte, cell int) {
	for l := from; l <= to; l++ {
		t[l] = cell
	}
}

func main() {
	cells := []*Cell{{Name: "cell 1", Version: 41}, {Name: "cell 2", Version: 41}, {Name: "cell 3", Version: 41}}
	r := &Router{Table: map[byte]int{}}
	Table(r.Table, 'A', 'H', 0)
	Table(r.Table, 'I', 'P', 1)
	Table(r.Table, 'Q', 'Z', 2)
	i, _ := r.Route("Kasia")
	fmt.Println("step 1-2: Kasia ->", cells[i].Name)
	done, stop := Deploy(cells, 42, map[string]float64{"cell 1": 0.0})
	fmt.Println("step 3: good version: finished", done, "stopped at", stop)
	for _, c := range cells {
		c.Version = 41
	}
	done, stop = Deploy(cells, 42, map[string]float64{"cell 1": 0.08})
	fmt.Printf("step 4: bad version: cell 1 errors 8%% > 1%%: finished %v, stopped at %s, versions %d %d %d\n", done, stop, cells[0].Version, cells[1].Version, cells[2].Version)
	fmt.Println("step 5:", Handle(cells[2], true), "|", Handle(cells[0], false), "|", Handle(cells[1], false))
	cells = append(cells, &Cell{Name: "cell 4", Version: 41})
	fmt.Println("step 6: new cell, same tested size:", cells[3].Name)
	bad := &Router{Table: map[byte]int{}}
	for _, name := range []string{"Ana", "Kasia", "Zoe"} {
		_, err := bad.Route(name)
		fmt.Println("step 7: bad table:", err)
	}
}
