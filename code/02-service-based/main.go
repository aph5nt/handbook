// Replays the service-based lesson: three coarse services on one StarRocks
// cluster, a database each, a direct read across owners that breaks on a
// column rename, and the published view that fixes it.
package main

import (
	"fmt"
	"sort"
)

// Object is a table or a view in one database of the cluster.
type Object struct {
	db      string
	columns []string
	view    bool
	over    string            // for a view: the table it reads
	rename  map[string]string // for a view: view column -> table column
}

type Cluster struct {
	owner   map[string]string // database -> the one service that owns it
	objects map[string]*Object
	reads   map[string]map[string][]string // service -> object -> columns its query names
	calls   map[string][]string            // service -> services it calls on a request
}

// readsAcross lists the tables a service reads that another service owns.
// A published view is a contract, so it does not count.
func (c *Cluster) readsAcross(s string) []string {
	var out []string
	for name := range c.reads[s] {
		o := c.objects[name]
		if c.owner[o.db] != s && !o.view { // a read across owners
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// renameColumn is the owner's change. It returns the readers it breaks.
func (c *Cluster) renameColumn(table, old, now string) []string {
	db := c.objects[table].db
	rename(c.objects[table].columns, old, now)
	rename(c.reads[c.owner[db]][table], old, now) // the owner updates its own queries

	for _, o := range c.objects { // and keeps its views working
		if o.view && o.over == table && o.rename[old] == old {
			o.rename[old] = now
		}
	}
	var broken []string
	for s, objs := range c.reads {
		for name, cols := range objs {
			if !c.resolves(name, cols) { // a column the query names is gone
				broken = append(broken, s+" reading "+name)
			}
		}
	}
	sort.Strings(broken)
	return broken
}

// resolves says whether every column a query names still exists.
func (c *Cluster) resolves(name string, cols []string) bool {
	o := c.objects[name]
	for _, col := range cols {
		if o.view {
			src, ok := o.rename[col]
			if !ok || !has(c.objects[o.over].columns, src) {
				return false
			}
		} else if !has(o.columns, col) {
			return false
		}
	}
	return true
}

// rename swaps old for now in a list of column names.
func rename(cols []string, old, now string) {
	for i, col := range cols {
		if col == old {
			cols[i] = now
		}
	}
}

func has(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func callCount(c *Cluster) int {
	n := 0
	for _, cs := range c.calls {
		n += len(cs)
	}
	return n
}

func newCluster() *Cluster {
	// one cluster, one owner per database, no calls
	return &Cluster{
		owner: map[string]string{"robinhood": "Indexer", "prices": "Oracle", "telemetry": "Validator"},
		objects: map[string]*Object{
			"robinhood.transfers": {db: "robinhood", columns: []string{"block", "amount"}},
			"prices.day_prices":   {db: "prices", columns: []string{"day", "usd"}},
			"telemetry.checks":    {db: "telemetry", columns: []string{"check", "ok"}},
		},
		reads: map[string]map[string][]string{
			"Indexer":   {"robinhood.transfers": {"block", "amount"}},
			"Oracle":    {"prices.day_prices": {"day", "usd"}},
			"Validator": {"telemetry.checks": {"check", "ok"}, "robinhood.transfers": {"block", "amount"}},
		},
		calls: map[string][]string{},
	}
}

func main() {
	fmt.Println("step 1-2: services 3, clusters 1, databases:")
	c := newCluster()
	for _, db := range []string{"robinhood", "prices", "telemetry"} {
		fmt.Printf("  %-9s owned by %s\n", db, c.owner[db])
	}
	fmt.Printf("step 3: calls between services on a request: %d\n", callCount(c))

	fmt.Printf("step 4: Validator reads across owners: %v\n", c.readsAcross("Validator"))
	broken := c.renameColumn("robinhood.transfers", "amount", "value")
	fmt.Printf("  Indexer renames amount to value. broken readers: %d %v\n", len(broken), broken)

	// the fix: a published view, and a grant on the view only
	c = newCluster()
	c.objects["robinhood.transfers_v"] = &Object{
		db: "robinhood", view: true, over: "robinhood.transfers",
		rename: map[string]string{"block": "block", "amount": "amount"},
	}
	c.reads["Validator"] = map[string][]string{
		"telemetry.checks":      {"check", "ok"},
		"robinhood.transfers_v": {"block", "amount"},
	}
	fmt.Printf("step 5: Validator reads across owners: %d %v\n", len(c.readsAcross("Validator")), c.readsAcross("Validator"))
	broken = c.renameColumn("robinhood.transfers", "amount", "value")
	v := c.objects["robinhood.transfers_v"]
	fmt.Printf("  Indexer renames amount to value, view maps amount -> %s. broken readers: %d\n", v.rename["amount"], len(broken))
	fmt.Printf("step 6: clusters to run: 1, services: %d\n", len(c.owner))
}
