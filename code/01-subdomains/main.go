// Subdomains: sort the parts of the business by edge, then spend by kind:
// build the core, build supporting parts cheap, buy generic parts.
package main

import "fmt"

// Part is one problem the business must solve.
type Part struct {
	Name           string
	WouldOutsource bool // "would we ever outsource this?"
	EveryCompany   bool // does every company have it?
	Incidents      int  // how loud it is: noise, not edge
}

// Kind answers the two questions in order.
func Kind(p Part) string {
	switch {
	case !p.WouldOutsource:
		return "core"
	case p.EveryCompany:
		return "generic"
	default:
		return "supporting"
	}
}

// Plan turns each kind into a budget line.
var Plan = map[string]string{"core": "build and own", "supporting": "build simply", "generic": "buy or run open source"}

// Seniors says where the senior engineers work.
// byEdge: on every core part. The failure: on the loudest part, by incidents.
func Seniors(parts []Part, byEdge bool) []string {
	var out []string
	if byEdge {
		for _, p := range parts {
			if Kind(p) == "core" {
				out = append(out, p.Name)
			}
		}
		return out
	}
	loudest := parts[0]
	for _, p := range parts {
		if p.Incidents > loudest.Incidents {
			loudest = p
		}
	}
	return []string{loudest.Name}
}

func main() {
	parts := []Part{
		{"money trace walk", false, false, 1}, {"risk rules", false, false, 0},
		{"block indexer", true, false, 2}, {"graph sync", true, false, 2},
		{"login", true, true, 9}, {"billing", true, true, 3},
	}
	for _, p := range parts {
		fmt.Printf("%-16s %-10s %s\n", p.Name, Kind(p), Plan[Kind(p)])
	}
	fmt.Println("seniors ranked by noise:", Seniors(parts, false))
	fmt.Println("seniors ranked by edge:", Seniors(parts, true))
}
