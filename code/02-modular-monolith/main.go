// Replays the modular monolith lesson: one indexer binary with three
// packages (pricing, dex, transfers), each owning its tables. Tab 1 runs the
// import check in the build. Tab 2 keeps the rule on a wiki page only.
package main

import (
	"fmt"
	"strings"
)

// Module is one Go package inside the one binary.
type Module struct {
	name    string
	tables  []string // tables this module owns
	imports []string // packages it imports
	reads   []string // tables it reads
}

type Binary struct {
	mods    []*Module
	enforce bool // true: the build runs the import check
}

// owner returns the module that owns table t.
func (b *Binary) owner(t string) string {
	for _, m := range b.mods {
		for _, x := range m.tables {
			if x == t {
				return m.name
			}
		}
	}
	return ""
}

// forbiddenImports applies Go's internal rule: a path under X/internal/
// may be imported only by code inside X.
func (b *Binary) forbiddenImports() (bad []string) {
	for _, m := range b.mods {
		for _, imp := range m.imports {
			i := strings.Index(imp, "/internal/")
			if i >= 0 && imp[:i] != m.name { // the importer is outside the tree
				bad = append(bad, m.name+" -> "+imp)
			}
		}
	}
	return
}

// build passes unless the check is on and finds a forbidden import.
func (b *Binary) build() (ok bool, bad []string) {
	if b.enforce { // off: nothing is checked
		bad = b.forbiddenImports()
	}
	return len(bad) == 0, bad
}

// forbiddenReads lists reads of a table owned by another module.
func (b *Binary) forbiddenReads() (bad []string) {
	for _, m := range b.mods {
		for _, t := range m.reads {
			if o := b.owner(t); o != m.name {
				bad = append(bad, m.name+" reads "+t)
			}
		}
	}
	return
}

// change lists the modules a change to table t breaks: every reader but the owner.
func (b *Binary) change(t string) []*Module {
	var broken []*Module
	for _, m := range b.mods {
		for _, r := range m.reads {
			if r == t && b.owner(t) != m.name {
				broken = append(broken, m)
			}
		}
	}
	return broken
}

// cascade follows one change: each broken module fixes itself by changing its
// own tables, which breaks their readers next.
func (b *Binary) cascade(t string) (order []string) {
	for queue := []string{t}; len(queue) > 0; queue = queue[1:] {
		for _, m := range b.change(queue[0]) { // every reader but the owner
			order = append(order, m.name+" (read "+queue[0]+")")
			queue = append(queue, m.tables...) // its fix changes its own tables
		}
	}
	return
}

// splitCost: modules that must be rewritten before m can move out.
func (b *Binary) splitCost(name string) (rewrite []string) {
	for _, m := range b.mods { // who reads name's tables from outside?
		for _, t := range m.reads {
			if m.name != name && b.owner(t) == name {
				rewrite = append(rewrite, m.name)
			}
		}
	}
	return
}

func (b *Binary) mod(name string) *Module {
	for _, m := range b.mods {
		if m.name == name {
			return m
		}
	}
	return nil
}

func newBinary(enforce bool) *Binary {
	return &Binary{enforce: enforce, mods: []*Module{
		{name: "pricing", tables: []string{"day_prices"}, reads: []string{"day_prices"}},
		{name: "dex", tables: []string{"swaps"}, imports: []string{"pricing"}, reads: []string{"swaps"}},
		{name: "transfers", tables: []string{"transfers"}, reads: []string{"transfers"}},
	}}
}

func printBuild(b *Binary) {
	ok, bad := b.build()
	switch {
	case !b.enforce:
		fmt.Println("  build: PASS (nothing checked)")
	case ok:
		fmt.Println("  build: PASS (no forbidden import)")
	default:
		fmt.Printf("  build: FAIL, forbidden import %v\n", bad)
	}
}

func main() {
	fmt.Println("tab 1: borders the build enforces")
	b := newBinary(true)
	fmt.Printf("  deploys: 1, modules: %d\n", len(b.mods))
	fmt.Printf("  forbidden reads: %d\n", len(b.forbiddenReads()))
	fmt.Println("  dex imports pricing and calls DayPrice(token, day)")
	printBuild(b)

	dex := b.mod("dex")
	fmt.Println("  shortcut: dex imports pricing/internal/store to read day_prices")
	dex.imports = append(dex.imports, "pricing/internal/store")
	dex.reads = append(dex.reads, "day_prices")
	printBuild(b)

	fmt.Println("  shortcut removed: dex calls DayPrice again")
	dex.imports = []string{"pricing"}
	dex.reads = []string{"swaps"}
	printBuild(b)
	fmt.Printf("  move pricing out later: modules to rewrite first %d %v\n", len(b.splitCost("pricing")), b.splitCost("pricing"))

	fmt.Println("tab 2: borders by convention only")
	c := newBinary(false)
	fmt.Printf("  forbidden reads: %d\n", len(c.forbiddenReads()))
	c.mod("dex").imports = append(c.mod("dex").imports, "store/pricing")
	c.mod("dex").reads = append(c.mod("dex").reads, "day_prices")
	printBuild(c)
	fmt.Printf("  after shortcut 1, forbidden reads: %d %v\n", len(c.forbiddenReads()), c.forbiddenReads())
	c.mod("transfers").reads = append(c.mod("transfers").reads, "swaps")
	fmt.Printf("  after shortcut 2, forbidden reads: %d %v\n", len(c.forbiddenReads()), c.forbiddenReads())
	fmt.Printf("  pricing renames a column in day_prices, broken in order: %v\n", c.cascade("day_prices"))
	fmt.Printf("  move pricing out later: modules to rewrite first %d %v\n", len(c.splitCost("pricing")), c.splitCost("pricing"))
}
