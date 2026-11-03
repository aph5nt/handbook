// Command cf replays the Cloudflare lesson: one request through a city, a
// backtracking matcher against a linear one on the pattern .*.*=.*, and the
// global push against a staged one. CPU figures are a model.
package main

import (
	"fmt"
	"regexp"
	"strings"
)

func Nearest(km map[string]int) (best string) {
	for city, d := range km {
		if best == "" || d < km[best] {
			best = city
		}
	}
	return best
}

func Handle(wafMatch, cached bool) string {
	if wafMatch {
		return "blocked"
	}
	page := "page with PLN prices" // the shop's Worker adds them
	if cached {
		return page + ", image from the cache"
	}
	return page + ", stock from the origin"
}

// Backtrack: a token ".*" tries every length. Steps counts every attempt.
func Backtrack(toks []string, s string, steps *int) bool {
	*steps++
	if len(toks) == 0 {
		return s == ""
	}
	if toks[0] == ".*" {
		for n := len(s); n >= 0; n-- {
			if Backtrack(toks[1:], s[n:], steps) {
				return true
			}
		}
		return false
	}
	return strings.HasPrefix(s, toks[0]) && Backtrack(toks[1:], s[len(toks[0]):], steps)
}

// Linear holds a set of live pattern positions and reads each char once.
func Linear(toks []string, s string) (ok bool, steps int) {
	close := func(set map[int]bool) {
		for i := range toks {
			if set[i] && toks[i] == ".*" {
				set[i+1] = true
			}
		}
	}
	set := map[int]bool{0: true}
	close(set)
	for _, c := range s {
		next := map[int]bool{}
		for i := range set {
			steps++
			if i < len(toks) && (toks[i] == ".*" || toks[i] == string(c)) {
				if toks[i] == ".*" {
					next[i] = true
				} else {
					next[i+1] = true
				}
			}
		}
		set = next
		close(set)
	}
	return set[len(toks)], steps
}

// CPU is a model: a base of 30 % plus the rule's cost against a step budget.
func CPU(wafOn bool, steps, budget int) int {
	if !wafOn {
		return 30
	}
	return min(100, 30+70*steps/budget)
}

// Staged pushes through stages and stops at the first hot one.
func Staged(stages []string, cpu func(string) int) (reached []string, stopped string) {
	for _, st := range stages {
		if cpu(st) > 80 {
			return reached, st
		}
		reached = append(reached, st)
	}
	return reached, ""
}

func main() {
	fmt.Println("edge step 1: Kasia reaches:", Nearest(map[string]int{"Warsaw": 5, "Frankfurt": 900, "Paris": 1400}))
	fmt.Println("edge step 3-5: image:", Handle(false, true), "; stock count:", Handle(false, false))

	toks := []string{".*", ".*", "=", ".*"}
	re := regexp.MustCompile(`.*.*=.*`)
	for _, n := range []int{10, 100, 1000, 2000} {
		text := strings.Repeat("x", n)
		steps := 0
		Backtrack(toks, text, &steps)
		ok, lin := Linear(toks, text)
		fmt.Printf("out step 3: length %4d: backtracking %8d steps, linear %5d steps, match %v, Go regexp match %v\n", n, steps, lin, ok, re.MatchString(text))
	}

	const budget = 100000
	old := 2005003
	_, lin := Linear(toks, strings.Repeat("x", 2000))
	bad := func(string) int { return CPU(true, old, budget) }
	good := func(string) int { return CPU(true, lin, budget) }
	fmt.Println("out step 4: the old push, every server in every city: CPU", bad("all"))
	fmt.Println("out step 5: kill switch, WAF off: CPU", CPU(false, old, budget))
	reached, stop := Staged([]string{"canary", "a few cities", "all"}, bad)
	fmt.Printf("out step 6: bad rule: finished %d stages, stopped at %s\n", len(reached), stop)
	reached, stop = Staged([]string{"canary", "a few cities", "all"}, good)
	fmt.Printf("out step 7: linear engine: CPU %d, finished %v\n", good("all"), reached)
}
