// Replays the strangler fig lesson: a facade moves four routes one at a time,
// compares in the shadow before each flip, and rolls back one route when it
// breaks. The second half replays the big-bang rewrite with the same facade.
package main

import "fmt"

type state int

const (
	onOld state = iota
	shadow
	onNew
)

// Facade holds one routing rule per path, and who owns each route's tables.
type Facade struct {
	routes map[string]state
	tables map[string]string
	newOK  map[string]func(req string) bool // does the new service serve this request?
}

// Handle routes one request. In the shadow state the old service answers the
// customer and the new one only runs for the comparison, as a dry run.
func (f *Facade) Handle(path, req string) (served string, ok, match bool) {
	switch f.routes[path] {
	case onNew:
		return "new", f.newOK[path](req), true
	case shadow:
		return "old", true, f.newOK[path](req)
	default:
		return "old", true, true
	}
}

// Compare runs requests through the shadow and counts the differences.
func (f *Facade) Compare(path string, reqs []string) (same, diff int) {
	f.routes[path] = shadow
	for _, r := range reqs {
		if _, _, match := f.Handle(path, r); match {
			same++
		} else {
			diff++
		}
	}
	return same, diff
}

// Flip moves a route and its tables to the new service, only with zero differences.
func (f *Facade) Flip(path string, diff int) bool {
	if diff != 0 {
		return false
	}
	f.routes[path] = onNew
	f.tables[path] = "new"
	return true
}

// Rollback is one rule at the facade: old answers again, new stays in the shadow.
func (f *Facade) Rollback(path string) {
	f.routes[path] = shadow
	f.tables[path] = "old"
}

// OnNew counts the routes the new services carry.
func (f *Facade) OnNew() int {
	n := 0
	for _, s := range f.routes {
		if s == onNew {
			n++
		}
	}
	return n
}

// SwitchOver flips every route at once. One failure rolls back every route,
// because they shipped as one change.
func (f *Facade) SwitchOver(live map[string]string) (failed int) {
	for path := range f.routes {
		f.routes[path] = onNew
	}
	for path, req := range live {
		if _, ok, _ := f.Handle(path, req); !ok {
			failed++
		}
	}
	if failed > 0 {
		for path := range f.routes {
			f.routes[path] = onOld
		}
	}
	return failed
}

func newFacade(payments func(string) bool) *Facade {
	always := func(string) bool { return true }
	return &Facade{
		routes: map[string]state{"catalog": onOld, "orders": onOld, "payments": onOld, "login": onOld},
		tables: map[string]string{"catalog": "old", "orders": "old", "payments": "old", "login": "old"},
		newOK:  map[string]func(string) bool{"catalog": always, "orders": always, "payments": payments, "login": always},
	}
}

func main() {
	f := newFacade(func(req string) bool { return req != "prepaid" })
	fmt.Printf("facade in front: routes on new services %d of 4\n", f.OnNew())

	same, diff := f.Compare("catalog", []string{"a", "b", "c", "d"})
	fmt.Printf("catalog in the shadow: %d same, %d different\n", same, diff)
	fmt.Printf("flip catalog: %v, on new %d of 4\n", f.Flip("catalog", diff), f.OnNew())

	same, diff = f.Compare("orders", []string{"a", "b", "c"})
	fmt.Printf("orders: %d same, %d different, flip %v, tables owned by %s, on new %d of 4\n",
		same, diff, f.Flip("orders", diff), f.tables["orders"], f.OnNew())

	same, diff = f.Compare("payments", []string{"visa", "mastercard"})
	fmt.Printf("payments, sample has no prepaid card: %d same, %d different, flip %v\n", same, diff, f.Flip("payments", diff))
	_, ok, _ := f.Handle("payments", "prepaid")
	fmt.Printf("live prepaid card on new payments: served ok = %v\n", ok)

	f.Rollback("payments")
	served, ok, _ := f.Handle("payments", "prepaid")
	fmt.Printf("rollback: prepaid served by %s, ok = %v, on new %d of 4\n", served, ok, f.OnNew())

	f.newOK["payments"] = func(string) bool { return true } // the team fixes the new service
	same, diff = f.Compare("payments", []string{"visa", "mastercard", "prepaid"})
	fmt.Printf("payments fixed, sample now has prepaid: %d same, %d different, flip %v\n", same, diff, f.Flip("payments", diff))
	same, diff = f.Compare("login", []string{"a", "b"})
	fmt.Printf("login on the set date: %d same, %d different, flip %v, on new %d of 4\n", same, diff, f.Flip("login", diff), f.OnNew())
	fmt.Printf("monolith receives no route: switch it off = %v\n", f.OnNew() == 4)

	fmt.Println("big-bang rewrite:")
	done, toCopy := 3, 2
	fmt.Printf("  rewrite done %d of 4 parts, plus %d new features to copy\n", done, toCopy)
	b := newFacade(func(req string) bool { return req != "new-checkout" })
	b.newOK["orders"] = func(req string) bool { return req != "gift-card" }
	failed := b.SwitchOver(map[string]string{"catalog": "a", "orders": "gift-card", "payments": "new-checkout", "login": "a"})
	fmt.Printf("  switch-over day: 4 routes flip at once, %d fail\n", failed)
	fmt.Printf("  roll back everything: routes on new %d of 4\n", b.OnNew())
}
