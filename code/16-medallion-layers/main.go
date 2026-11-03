// Medallion layers: bronze keeps raw input, silver holds typed facts,
// gold answers one business question. Replays the lesson's story.
package main

import "fmt"

// Raw is a log exactly as captured from the chain node.
type Raw struct {
	Block, Log int
	Exchange   string
	RawAmount  int // integer units, decimals not applied
	Decimals   int
	OK         bool
}

// Fact is a typed swap in silver.
type Fact struct {
	Block, Log int
	Exchange   string
	Amount     float64
	OK         bool
}

type Pipeline struct {
	nodeCalls int
	bronze    []Raw
	silver    []Fact
	gold      map[string]float64
}

// Capture stores a block's raw logs in bronze, untouched.
func (p *Pipeline) Capture(logs []Raw) {
	p.nodeCalls++ // one request to the chain node per block
	p.bronze = append(p.bronze, logs...)
}

// decode turns a raw log into a typed fact. v1 forgets decimals for exchange B.
func decode(r Raw, version int) Fact {
	div := 1.0
	for k := 0; k < r.Decimals; k++ {
		div *= 10
	}
	if version == 1 && r.Exchange == "B" {
		div = div / 10 // the decoder bug: one decimal place lost
	}
	return Fact{r.Block, r.Log, r.Exchange, float64(r.RawAmount) / div, r.OK}
}

// BuildSilver decodes bronze and removes duplicates by block and log index.
func (p *Pipeline) BuildSilver(version int) {
	p.silver = nil
	seen := map[[2]int]bool{}
	for _, r := range p.bronze {
		f := decode(r, version)
		if seen[[2]int{f.Block, f.Log}] {
			continue // the same event captured twice
		}
		seen[[2]int{f.Block, f.Log}] = true
		p.silver = append(p.silver, f)
	}
}

// BuildGold applies the business rule once: volume of successful swaps.
func (p *Pipeline) BuildGold() {
	p.gold = map[string]float64{}
	for _, f := range p.silver {
		if f.OK {
			p.gold[f.Exchange] += f.Amount
		}
	}
}

// boardOnSilver is the trap: each board writes its own rule.
func boardOnSilver(silver []Fact, skipFailed bool) (total float64) {
	for _, f := range silver {
		if skipFailed && !f.OK {
			continue
		}
		total += f.Amount
	}
	return total
}

func main() {
	p := &Pipeline{}
	blocks := [][]Raw{
		{{100, 0, "A", 100, 1, true}, {100, 0, "A", 100, 1, true}}, // captured twice
		{{101, 0, "B", 50, 1, true}},
		{{102, 0, "A", 80, 1, false}}, // a failed swap
		{{103, 0, "B", 70, 1, true}},
		{{104, 0, "A", 200, 1, true}},
	}
	for _, b := range blocks {
		p.Capture(b)
	}
	fmt.Printf("bronze: %d raw logs from %d blocks, node calls %d\n", len(p.bronze), len(blocks), p.nodeCalls)
	p.BuildSilver(1)
	fmt.Printf("silver v1: %d facts after removing duplicates\n", len(p.silver))
	p.BuildGold()
	fmt.Printf("gold v1: A %.0f, B %.0f\n", p.gold["A"], p.gold["B"])
	fmt.Printf("trap, boards on silver: board 1 %.0f, board 2 %.0f\n", boardOnSilver(p.silver, false), boardOnSilver(p.silver, true))

	calls := p.nodeCalls
	p.BuildSilver(2) // the decoder fix, replayed from bronze
	p.BuildGold()
	fmt.Printf("after fix from bronze: gold A %.0f, B %.0f, extra node calls %d\n", p.gold["A"], p.gold["B"], p.nodeCalls-calls)
	fmt.Printf("fixed boards on silver: board 1 %.0f, board 2 %.0f, gold total %.0f\n",
		boardOnSilver(p.silver, false), boardOnSilver(p.silver, true), p.gold["A"]+p.gold["B"])

	// without bronze: the fix must fetch every block from the node again
	q := &Pipeline{}
	for _, b := range blocks {
		q.Capture(b)
	}
	q.bronze = nil // thrown away to save space
	for _, b := range blocks {
		q.Capture(b) // every block again
	}
	fmt.Printf("no bronze: node calls for the fix %d (every block again)\n", q.nodeCalls-len(blocks))
}
