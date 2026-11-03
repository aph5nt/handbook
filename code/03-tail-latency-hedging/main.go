// Tail latency and hedged requests. It replays the lesson: a page asks 100
// shards and waits for the slowest. A real hedged read runs with goroutines,
// then many simulated pages give the percentages.
package main

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"
)

const (
	hedgeAfter = 20 * time.Millisecond // about p95 of one shard call
	budget     = 0.05                  // at most 5 extra calls per 100
)

// Read is one call to one copy of a shard.
type Read func(ctx context.Context) string

// ReadAll is the page: it waits for every shard, so the slowest decides.
func ReadAll(reads []Read) time.Duration {
	start := time.Now()
	var wg sync.WaitGroup
	for _, r := range reads {
		wg.Add(1)
		go func(r Read) { defer wg.Done(); r(context.Background()) }(r)
	}
	wg.Wait()
	return time.Since(start)
}

// Guard decides if a slow call may get a second copy.
type Guard struct {
	calls, hedges int
}

// Allow refuses a write, and refuses a hedge beyond the budget.
func (g *Guard) Allow(idempotent bool) bool {
	if !idempotent {
		return false // never send a payment twice
	}
	if float64(g.hedges+1) > budget*float64(g.calls) {
		return false // the budget is spent: wait for the primary
	}
	g.hedges++
	return true
}

// Hedged reads the primary, and after a short wait also a replica. The first
// answer wins and the other call is cancelled.
func Hedged(primary, replica Read, idempotent bool, g *Guard) (string, int) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancels the loser
	answers := make(chan string, 2)
	g.calls++
	go func() { answers <- primary(ctx) }()
	select {
	case a := <-answers:
		return a, 1 // fast path: no hedge
	case <-time.After(hedgeAfter):
	}
	if !g.Allow(idempotent) {
		return <-answers, 1
	}
	go func() { answers <- replica(ctx) }()
	return <-answers, 2
}

func sleeper(d time.Duration, name string) Read {
	return func(ctx context.Context) string {
		select {
		case <-time.After(d):
			return name
		case <-ctx.Done():
			return "cancelled"
		}
	}
}

func round10(d time.Duration) int { return int(d.Milliseconds()+5) / 10 * 10 }

// One shard call: 95 % take 10 ms, 4 % take 30 ms, 1 % are in a 1 s pause.
func latency(r *rand.Rand) int {
	switch x := r.Float64(); {
	case x < 0.95:
		return 10
	case x < 0.99:
		return 30
	}
	return 1000
}

func percentile(v []int, p float64) int {
	sort.Ints(v)
	return v[int(p*float64(len(v)-1))]
}

func main() {
	fmt.Println("== One page, shard 14 in a pause")
	reads := make([]Read, 100)
	for i := range reads {
		reads[i] = sleeper(10*time.Millisecond, "fast")
	}
	reads[13] = sleeper(time.Second, "shard 14")
	fmt.Printf("no hedge: page takes about %d ms\n", round10(ReadAll(reads)))

	g := &Guard{calls: 100} // the page's other 99 calls count toward the budget
	start := time.Now()
	a, calls := Hedged(sleeper(time.Second, "shard 14"), sleeper(10*time.Millisecond, "replica 14"), true, g)
	fmt.Printf("hedged: %s answers in about %d ms, %d calls\n", a, round10(time.Since(start)), calls)

	fmt.Printf("chance no shard of 100 is slow: 0.99^100 = %.2f\n", math.Pow(0.99, 100))

	fmt.Println("== 10,000 simulated pages")
	r := rand.New(rand.NewSource(1))
	var plain, hedged []int
	slowPages, shardCalls, hedges := 0, 0, 0
	for p := 0; p < 10000; p++ {
		worst, worstH, paused := 0, 0, false
		for s := 0; s < 100; s++ {
			l := latency(r)
			shardCalls++
			paused = paused || l == 1000
			worst = max(worst, l)
			if l > 20 { // slower than the hedge delay: send a second copy
				hedges++
				l = min(l, 20+latency(r))
			}
			worstH = max(worstH, l)
		}
		if paused {
			slowPages++
		}
		plain, hedged = append(plain, worst), append(hedged, worstH)
	}
	fmt.Printf("pages meeting a paused shard: %.0f %%\n", 100*float64(slowPages)/10000)
	fmt.Printf("hedges per 100 calls: %.1f\n", 100*float64(hedges)/float64(shardCalls))
	fmt.Printf("page p50: no hedge %d ms, hedged %d ms\n", percentile(plain, .5), percentile(hedged, .5))

	fmt.Println("== Hedging the wrong thing")
	all := 0
	for s := 0; s < 100; s++ {
		all += 2 // no delay: every call goes to two copies at once
	}
	fmt.Printf("hedge every call at once: 100 calls become %d\n", all)
	g = &Guard{calls: 100}
	fmt.Printf("payment may be hedged: %v\n", g.Allow(false))
	g = &Guard{calls: 100, hedges: 5}
	fmt.Printf("sixth hedge in 100 calls allowed: %v\n", g.Allow(true))
}
