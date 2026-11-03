// Replays the micro-frontends lesson: a shell app loads three team-owned
// parts by route. Each part deploys alone. Then it counts what a user
// downloads when every part bundles its own React and buttons, and when the
// shell shares one copy of each. Sizes are example sizes in KB.
package main

import (
	"fmt"
	"sort"
)

type Part struct {
	team    string
	version string
	codeKB  int
	bundles []string // libraries packed inside this part's bundle
	buttons string   // whose buttons it draws
}

type Shell struct {
	routes map[string]*Part // the shell owns the routes, login and layout
	shared []string         // libraries the shell loads once, for every part
}

var libKB = map[string]int{"react": 140, "buttons": 40} // example sizes

// load is what the shell does on a route: find the part, load its bundle.
func (s *Shell) load(route string) *Part { return s.routes[route] }

// deploy ships a new version of one part. Nothing else is rebuilt.
func (s *Shell) deploy(route, version string) (rebuilt []string) {
	s.routes[route].version = version
	return []string{s.routes[route].team}
}

// session visits every route once and adds up the KB downloaded.
func (s *Shell) session(routes []string) (kb, reactCopies int, looks map[string]bool) {
	looks = map[string]bool{}
	for _, l := range s.shared {
		kb += libKB[l] // loaded once by the shell
		if l == "react" {
			reactCopies++
		}
	}
	for _, r := range routes {
		p := s.load(r)
		kb += p.codeKB
		for _, l := range p.bundles {
			kb += libKB[l] // a private copy inside the part
			if l == "react" {
				reactCopies++
			}
		}
		looks[p.buttons] = true
	}
	return kb, reactCopies, looks
}

// share moves React and the buttons out of every part into the shell.
func (s *Shell) share() {
	s.shared = []string{"react", "buttons"}
	for _, p := range s.routes {
		p.bundles = nil
		p.buttons = "design system"
	}
}

// worthSplitting: split only when several teams keep blocking each other in
// one codebase. It solves a team problem, not a technical one.
func worthSplitting(teams int, blocking bool) bool {
	return teams > 1 && blocking
}

func main() {
	sh := &Shell{routes: map[string]*Part{
		"/risk":    {"graph team", "v1", 60, []string{"react", "buttons"}, "graph team"},
		"/billing": {"payments team", "v1", 45, []string{"react", "buttons"}, "payments team"},
		"/account": {"identity team", "v1", 30, []string{"react", "buttons"}, "identity team"},
	}}
	routes := []string{"/risk", "/billing", "/account"}
	for _, r := range routes {
		fmt.Printf("%-9s loads the part of the %s\n", r, sh.load(r).team)
	}
	fmt.Printf("Tuesday: payments ships billing v2, rebuilt: %v\n", sh.deploy("/billing", "v2"))

	kb, react, looks := sh.session(routes)
	fmt.Printf("before: one session of 3 pages downloads %d KB, %d copies of React, %d kinds of button\n", kb, react, len(looks))
	sh.share()
	kb, react, looks = sh.session(routes)
	var names []string
	for k := range looks {
		names = append(names, k)
	}
	sort.Strings(names)
	fmt.Printf("after:  one session of 3 pages downloads %d KB, %d copy of React, %d kind of button %v\n", kb, react, len(looks), names)
	fmt.Printf("worth splitting? 1 team: %v, 3 teams blocking each other: %v\n", worthSplitting(1, false), worthSplitting(3, true))
}
