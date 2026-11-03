// A graph sync in miniature: read transfer facts from the warehouse in
// pages of rows after a high-water mark, merge them as FLOWS_TO links,
// and rebuild the graph from the facts when the facts change.
package main

import (
	"fmt"
	"sort"
	"strings"
)

// The statements the real sync runs against StarRocks and DozerDB.
const pageSQL = `SELECT block, sender, receiver, amount FROM transfers
WHERE block > ? ORDER BY block LIMIT ?            -- after the mark, by rows`

const mergeCypher = `UNWIND $rows AS r
MERGE (a:Address {id: r.sender})
MERGE (b:Address {id: r.receiver})
MERGE (a)-[f:FLOWS_TO {block: r.block}]->(b) SET f.amount = r.amount`

type Transfer struct {
	Block         int
	Sender, Recvr string
	Amount        float64
}

type Graph struct{ links map[string]float64 } // "A>B" -> amount

type Sync struct {
	mark     int // high-water mark: the last block merged into this layer
	pageRows int // pages are sized by rows, not by blocks
}

// readPage is the pageSQL query over the warehouse rows.
func readPage(facts []Transfer, mark, limit int) []Transfer {
	var page []Transfer
	for _, t := range facts { // facts are ordered by block
		if t.Block > mark && len(page) < limit {
			page = append(page, t)
		}
	}
	return page
}

// Step reads one page after the mark, merges it, then moves the mark.
func (s *Sync) Step(facts []Transfer, g *Graph) int {
	page := readPage(facts, s.mark, s.pageRows)
	for _, t := range page {
		g.links[t.Sender+">"+t.Recvr] = t.Amount // MERGE: idempotent
	}
	if len(page) > 0 {
		s.mark = page[len(page)-1].Block // only after the merge
	}
	return len(page)
}

// Rebuild empties the layer and resets the mark: the graph follows SQL.
func (s *Sync) Rebuild(facts []Transfer, g *Graph) {
	g.links = map[string]float64{}
	s.mark = 99
	for s.Step(facts, g) > 0 {
	}
}

// reach walks FLOWS_TO links outward up to n hops.
func (g *Graph) reach(from string, n int) []string {
	seen, frontier := map[string]bool{from: true}, []string{from}
	for hop := 0; hop < n; hop++ {
		var next []string
		for _, a := range frontier {
			for l := range g.links {
				p := strings.Split(l, ">")
				if p[0] == a && !seen[p[1]] {
					seen[p[1]] = true
					next = append(next, p[1])
				}
			}
		}
		frontier = next
	}
	var out []string
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (g *Graph) keys() string {
	var ks []string
	for k := range g.links {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return strings.Join(ks, " ")
}

func main() {
	facts := []Transfer{{100, "A", "B", 5}, {101, "B", "C", 3}, {102, "C", "D", 2}, {103, "A", "E", 1}}
	g := &Graph{links: map[string]float64{}}
	s := &Sync{mark: 99, pageRows: 2}
	n := s.Step(facts, g)
	fmt.Printf("page 1: %d rows, links %s, mark %d\n", n, g.keys(), s.mark)
	n = s.Step(facts, g)
	fmt.Printf("page 2: %d rows, links %s, mark %d\n", n, g.keys(), s.mark)
	fmt.Println("graph search from A, 3 hops:", g.reach("A", 3))

	facts[2].Amount = 2.5 // the decoder fix re-indexed block 102
	s.Rebuild(facts, g)
	fmt.Printf("rebuilt: links %s, C>D amount %.1f, mark %d\n", g.keys(), g.links["C>D"], s.mark)

	g.links["E>D"] = 4 // a hand edit straight into the graph
	fmt.Println("after hand edit, links:", g.keys())
	s.Rebuild(facts, g)
	_, kept := g.links["E>D"]
	fmt.Printf("after the next rebuild, E>D present: %v\n", kept)

	facts = append(facts, Transfer{104, "E", "D", 4}) // the fix: a SQL row first
	s.Step(facts, g)
	fmt.Printf("with a SQL row for block 104: links %s, mark %d\n", g.keys(), s.mark)
}
