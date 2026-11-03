// Replays the evolutionary architecture lesson: three fitness functions in
// the build, a hundred pull requests inside the limits, one that crosses a
// limit and is fixed by moving the work, and a limit raised to get a release out.
package main

import "fmt"

// Repo is what the build measures.
type Repo struct {
	unitSuites   map[string]int // unit tier: suite -> seconds
	integSuites  map[string]int // integration tier: suite -> seconds
	biggestFile  int            // lines of the biggest file
	domainImport int            // imports from domain packages into infrastructure
}

// Fitness is a number with a limit the build checks.
type Fitness struct {
	name    string
	limit   int
	measure func(*Repo) int
}

func sum(m map[string]int) int {
	t := 0
	for _, v := range m {
		t += v
	}
	return t
}

// build runs every fitness function and fails on the first one over its limit.
func build(r *Repo, fs []*Fitness) (bool, string) {
	for _, f := range fs {
		if v := f.measure(r); v > f.limit {
			return false, fmt.Sprintf("%s: %d over the limit %d", f.name, v, f.limit)
		}
	}
	return true, "all checks pass"
}

// raiseLimit is the guard on the guards: a limit moves only with a reviewed reason.
func raiseLimit(f *Fitness, to int, reviewedReason string) bool {
	if reviewedReason == "" {
		return false
	}
	f.limit = to
	return true
}

// barPx turns a value into the gauge width in the lesson: the limit sits at 498 px.
func barPx(v, limit int) int { return v * 498 / limit }

func main() {
	r := &Repo{
		unitSuites:  map[string]int{"decode": 40, "math": 35, "split": 33},
		integSuites: map[string]int{"starrocks": 410},
		biggestFile: 780,
	}
	unit := &Fitness{"unit tier seconds", 180, func(r *Repo) int { return sum(r.unitSuites) }}
	size := &Fitness{"biggest file lines", 1500, func(r *Repo) int { return r.biggestFile }}
	dom := &Fitness{"domain imports of infrastructure", 0, func(r *Repo) int { return r.domainImport }}
	fs := []*Fitness{unit, size, dom}

	show := func(step string) {
		ok, why := build(r, fs)
		fmt.Printf("%s: unit %d s (bar %d), biggest file %d lines (bar %d), domain imports %d. build ok=%v: %s\n",
			step, unit.measure(r), barPx(unit.measure(r), 180), r.biggestFile, barPx(r.biggestFile, 1500), r.domainImport, ok, why)
	}
	fmt.Printf("step 1-2: fitness functions: %d, limits 180 s, 1500 lines, 0 imports\n", len(fs))
	show("start")

	// A hundred pull requests: 22 add a 1 s unit test, 70 add 3 lines, 8 change neither.
	for i := 0; i < 100; i++ {
		switch {
		case i < 22:
			r.unitSuites["decode"]++
		case i < 92:
			r.biggestFile += 3
		}
		if ok, _ := build(r, fs); !ok {
			fmt.Println("a pull request failed the build")
		}
	}
	show("step 3: after 100 pull requests")

	r.unitSuites["replay"] = 72 // a slow suite lands in the unit tier
	show("step 4: replay suite in the unit tier")

	delete(r.unitSuites, "replay") // the fix moves the work, not the limit
	r.integSuites["replay"] = 72
	show("step 5: replay moved")
	fmt.Printf("  integration tier %d s of its 600 s budget\n", sum(r.integSuites))

	r.biggestFile = 1520 // months later
	show("step 6: a file passes the limit")
	size.limit = 3000 // edited by hand to get a release out
	ok, _ := build(r, fs)
	fmt.Printf("  limit raised to %d, build ok=%v\n", size.limit, ok)
	r.biggestFile = 1800 // then again
	ok, _ = build(r, fs)
	fmt.Printf("  file grows to %d lines (bar %d against the old limit), build ok=%v. guard gone\n", r.biggestFile, barPx(r.biggestFile, 1500), ok)

	size.limit = 1500 // the guard: no reviewed reason, no change
	moved := raiseLimit(size, 3000, "")
	fmt.Printf("fix: raise limit with no reviewed reason: moved=%v, limit %d\n", moved, size.limit)
}
