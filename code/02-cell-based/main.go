// Replays the cell-based lesson: 2,600 tenants in three cells behind a thin
// router, a bad release in one cell, its rollback, and a shared global
// login service that takes every cell down.
package main

import "fmt"

// Cell is a full copy of the stack: API, workers and its own database.
type Cell struct {
	name     string
	from, to byte // first letters of the tenants it serves
	version  string
	login    string // "global" or "own"
}

type System struct {
	cells       []*Cell
	bad         map[string]bool // versions with a bug
	globalLogin bool            // is the shared login service up?
	tenants     []string
}

// route is the thin router: tenant to cell, nothing else.
func (s *System) route(tenant string) *Cell {
	for _, c := range s.cells {
		if tenant[0] >= c.from && tenant[0] <= c.to {
			return c
		}
	}
	return nil
}

// serve answers one request inside the tenant's cell.
func (s *System) serve(tenant string) bool {
	c := s.route(tenant)
	if c.login == "global" && !s.globalLogin { // shared by every cell
		return false
	}
	return !s.bad[c.version]
}

// blastRadius counts the tenants whose requests fail.
func (s *System) blastRadius() int {
	hit := 0
	for _, t := range s.tenants {
		if !s.serve(t) {
			hit++
		}
	}
	return hit
}

func (s *System) report(step string) {
	hit := s.blastRadius()
	fmt.Printf("%s: tenants hit %d of %d (%.1f percent)\n", step, hit, len(s.tenants), 100*float64(hit)/float64(len(s.tenants)))
}

func main() {
	s := &System{
		cells: []*Cell{
			{"cell 1", 'A', 'H', "v1", "global"},
			{"cell 2", 'I', 'P', "v1", "global"},
			{"cell 3", 'Q', 'Z', "v1", "global"},
		},
		bad:         map[string]bool{"v2": true},
		globalLogin: true,
	}
	for l := byte('A'); l <= 'Z'; l++ { // 100 tenants per first letter
		for i := 1; i <= 100; i++ {
			s.tenants = append(s.tenants, fmt.Sprintf("%c%03d", l, i))
		}
	}
	count := map[string]int{}
	for _, t := range s.tenants {
		count[s.route(t).name]++
	}
	fmt.Printf("step 1: %d tenants. cell 1 %d, cell 2 %d, cell 3 %d\n", len(s.tenants), count["cell 1"], count["cell 2"], count["cell 3"])
	fmt.Printf("step 2: route(K042) -> %s, route(T007) -> %s\n", s.route("K042").name, s.route("T007").name)

	s.cells[0].version = "v2" // release to one cell first
	fmt.Println("step 3: cell 1 on v2, cells 2 and 3 on v1")
	s.report("step 4: v2 has a bug")

	s.cells[0].version = "v1" // roll back one cell
	s.report("step 5: cell 1 rolled back")

	s.globalLogin = false // the trap: one login service for every cell
	s.report("step 6: global login down")

	for _, c := range s.cells { // the fix: login inside each cell
		c.login = "own"
	}
	s.report("fix: login inside each cell, global login still down")
}
