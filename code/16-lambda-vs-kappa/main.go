// Lambda versus kappa: the fee rule changes from v1 (0.30 per transfer) to
// v2 (0.25). Lambda updates the batch code and misses the speed code. Kappa
// has one code path and replays the chain from block 1.
package main

import "fmt"

type Block struct {
	Height int
	Day    int // 1 = yesterday, 2 = today
}

// FeeRule computes the fee of one transfer of 100 tokens.
type FeeRule func() float64

func v1() float64 { return 0.30 }
func v2() float64 { return 0.25 }

func chain() []Block {
	var bs []Block
	for h := 1; h <= 6; h++ {
		bs = append(bs, Block{Height: h, Day: (h-1)/3 + 1})
	}
	return bs
}

// Lambda: a batch layer and a speed layer, two programs, merged at read.
type Lambda struct {
	batchCode, speedCode FeeRule
	batchView, speedView map[int]float64 // fee by block height
}

func (l *Lambda) NightlyBatch(bs []Block, upToDay int) {
	for _, b := range bs {
		if b.Day <= upToDay {
			l.batchView[b.Height] = l.batchCode()
			delete(l.speedView, b.Height) // the speed copy of that period is dropped
		}
	}
}

func (l *Lambda) OnBlock(b Block) { l.speedView[b.Height] = l.speedCode() }

func (l *Lambda) Query(h int) float64 {
	if v, ok := l.batchView[h]; ok {
		return v
	}
	return l.speedView[h] // merge at read
}

// Kappa: one processor over a replayable log, writing one table.
type Kappa struct {
	rule  FeeRule
	table map[int]float64
}

func (k *Kappa) Run(bs []Block, from int) {
	for _, b := range bs {
		if b.Height >= from {
			k.table[b.Height] = k.rule() // upsert by block height
		}
	}
}

func main() {
	bs := chain()
	// Lambda: the rule changes, only the batch code is updated
	l := &Lambda{batchCode: v2, speedCode: v1, batchView: map[int]float64{}, speedView: map[int]float64{}}
	l.NightlyBatch(bs, 1) // last night: yesterday is batch
	for _, b := range bs {
		if b.Day == 2 {
			l.OnBlock(b) // today: speed layer, still v1
		}
	}
	fmt.Printf("lambda: yesterday's transfer fee %.2f, today's transfer fee %.2f (two numbers)\n", l.Query(3), l.Query(4))

	// Kappa: fix the one program to v2 and replay from block 1 into new tables
	k := &Kappa{rule: v2, table: map[int]float64{}}
	k.Run(bs, 1)
	fmt.Printf("kappa replay from block 1: yesterday %.2f, today %.2f (one rule)\n", k.table[3], k.table[4])

	// The failure: a source that forgets. An API that keeps 30 days.
	const keepDays, wantDays = 30, 365
	missing := wantDays - keepDays
	fmt.Printf("kappa over an API that keeps %d days: replay of %d days has no input for %d days\n", keepDays, wantDays, missing)
}
