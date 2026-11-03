// Replays the Conway's law lesson. A coupling joins two services. When one
// side changes, every service coupled to it without a versioned contract
// breaks, and every team that owns one of them must join the fix. Two teams
// means a meeting. The program counts services per team, cross-team
// couplings and the teams each change pulls in.
package main

import (
	"fmt"
	"sort"
)

type Coupling struct {
	From, To string // To depends on From
	Kind     string // "shared types", "shared table", "run order", "contract"
	Drawn    bool   // shown on the architecture diagram
}

type Org struct {
	owner     map[string]string // service -> team
	couplings []Coupling
}

// breaks lists the services a change to s breaks. A versioned contract
// absorbs the change, every other kind of coupling passes it on.
func (o *Org) breaks(s string) []string {
	var out []string
	for _, c := range o.couplings {
		if c.From == s && c.Kind != "contract" {
			out = append(out, c.To)
		}
	}
	return out
}

// teamsPulledIn is who must act on a change to s: its owner and the owners
// of every service it breaks. More than one team means a meeting.
func (o *Org) teamsPulledIn(s string) []string {
	teams := map[string]bool{o.owner[s]: true}
	for _, b := range o.breaks(s) {
		teams[o.owner[b]] = true
	}
	return sortedKeys(teams)
}

// crossTeam counts couplings between two teams, and the hidden ones among them.
func (o *Org) crossTeam() (all, hidden int) {
	for _, c := range o.couplings {
		if o.owner[c.From] != o.owner[c.To] && c.Kind != "contract" {
			all++
			if !c.Drawn {
				hidden++
			}
		}
	}
	return all, hidden
}

// perTeam counts the services each team deploys, watches and is paged for.
func (o *Org) perTeam() map[string]int {
	n := map[string]int{}
	for _, t := range o.owner {
		n[t]++
	}
	return n
}

func sortedKeys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func change(o *Org, s string) {
	t := o.teamsPulledIn(s)
	fmt.Printf("  change %s: breaks %v, teams pulled in %v", s, o.breaks(s), t)
	if len(t) > 1 {
		fmt.Print(" -> a meeting")
	}
	fmt.Println()
}

func orgTab() {
	fmt.Println("tab 1: the org moves, the code follows")
	o := &Org{
		owner:     map[string]string{"indexer": "Ingest", "graph sync": "Ingest"},
		couplings: []Coupling{{"indexer", "graph sync", "shared types", true}},
	}
	all, _ := o.crossTeam()
	fmt.Printf(" one team: cross-team couplings %d\n", all)
	change(o, "indexer")

	o.owner["graph sync"] = "Graph" // the org moves first
	all, _ = o.crossTeam()
	fmt.Printf(" two teams, same code: cross-team couplings %d\n", all)
	change(o, "indexer")

	o.couplings[0].Kind = "contract" // shared types become a published schema
	all, _ = o.crossTeam()
	fmt.Printf(" published schema v1: cross-team couplings %d\n", all)
	change(o, "indexer")
}

func twelveTab() {
	fmt.Println("tab 2: twelve services, three teams")
	o := &Org{owner: map[string]string{}}
	teams := map[string][]string{
		"A": {"login", "profile", "wallets", "limits"},
		"B": {"prices", "swaps", "pools", "tokens"},
		"C": {"graph", "alerts", "reports", "exports"},
	}
	for t, svcs := range teams {
		for _, s := range svcs {
			o.owner[s] = t
		}
	}
	o.couplings = []Coupling{
		{"wallets", "tokens", "shared table", false}, // both write one table
		{"pools", "reports", "run order", false},     // reports must run after pools
	}
	fmt.Printf(" services per team: %v\n", o.perTeam())
	all, hidden := o.crossTeam()
	fmt.Printf(" cross-team couplings %d, hidden (not drawn) %d\n", all, hidden)
	change(o, "wallets")
	change(o, "pools")

	// the fix: merge into services each team can own
	merge := map[string]string{"profile": "login", "limits": "wallets", "swaps": "prices",
		"tokens": "pools", "alerts": "graph", "exports": "reports"}
	for from, into := range merge {
		delete(o.owner, from)
		for i := range o.couplings {
			if o.couplings[i].To == from {
				o.couplings[i].To = into
			}
		}
	}
	o.couplings[0] = Coupling{"wallets", "pools", "contract", true} // table owned by A, B reads A's API
	o.couplings[1].Drawn = true                                     // run order drawn on the diagram
	fmt.Printf(" after the merge: services per team: %v\n", o.perTeam())
	all, hidden = o.crossTeam()
	fmt.Printf(" cross-team couplings %d, hidden %d\n", all, hidden)
	change(o, "wallets")
	change(o, "pools")
}

func main() {
	orgTab()
	twelveTab()
}
