// Back-pressure. It replays the lesson: an indexer offers 2,000 rows a second,
// a loader writes 1,000. Without a limit the pod dies at 50 s. With a limit of
// 10,000 the indexer runs at the loader's pace. Then a real Go channel shows
// the same rule, and a pull reader keeps the backlog on disk.
package main

import "fmt"

const (
	indexerRate = 2000 // rows per second the indexer can produce
	loaderRate  = 1000 // rows per second the loader can write
	baseMem     = 0.10 // share of the pod limit used with an empty queue
	rowMem      = 0.9 / 50000
)

// Rule says what happens to a row when the queue is full.
type Rule int

const (
	Block Rule = iota // the producer waits: nothing is lost
	Shed              // the row is dropped and counted
)

// Pipeline is the in-memory queue between the indexer and the loader.
type Pipeline struct {
	queue, limit int // limit 0 means no limit
	rule         Rule
	rate, shed   int
	killed       bool
}

// Second runs one second. The loader's slowness reaches the indexer here.
func (p *Pipeline) Second() {
	offered := indexerRate
	taken := min(loaderRate, p.queue+offered)
	if p.limit > 0 {
		free := p.limit - p.queue + taken
		if offered > free { // the queue is full: apply the rule
			if p.rule == Shed {
				p.shed += offered - free // dropped and counted
			}
			offered = free // Block: the indexer waits for a free slot
		}
	}
	p.queue += offered - taken
	p.rate = offered
	if p.Memory() >= 1 {
		p.killed = true // over the pod limit: every queued row is lost
	}
}

// Memory is the share of the pod limit in use.
func (p *Pipeline) Memory() float64 { return baseMem + float64(p.queue)*rowMem }

// ChannelDemo is the same rule in real Go: a buffered channel. A send blocks
// when the buffer is full, so the buffer can never grow past its size.
func ChannelDemo(rows, size int) (delivered, maxLen int) {
	queue := make(chan int, size)
	go func() {
		for r := 0; r < rows; r++ {
			queue <- r // blocks while the channel is full
			maxLen = max(maxLen, len(queue))
		}
		close(queue)
	}()
	for range queue {
		delivered++ // the consumer frees one slot per row
	}
	return delivered, maxLen
}

// Log is a pull source, like Kafka: rows wait on disk, not in memory.
type Log struct {
	appended, consumed int
}

// Pull gives the reader only what it asked for. The rest stays in the log.
func (l *Log) Pull(n int) int {
	got := min(n, l.appended-l.consumed)
	l.consumed += got
	return got
}

func main() {
	fmt.Println("== No limit")
	p := &Pipeline{}
	for t := 1; t <= 50; t++ {
		p.Second()
		if t == 1 || t == 10 || t == 40 || t == 50 {
			fmt.Printf("t=%2d s: queue %6d, memory %3.0f %%, killed %v\n", t, p.queue, p.Memory()*100, p.killed)
		}
	}

	fmt.Println("== Limit 10,000, rule: block")
	p = &Pipeline{limit: 10000, rule: Block}
	for t := 1; t <= 60; t++ {
		p.Second()
		if t == 5 || t == 10 || t == 11 || t == 60 {
			fmt.Printf("t=%2d s: queue %6d, indexer %d rows/s, memory %3.0f %%\n", t, p.queue, p.rate, p.Memory()*100)
		}
	}

	fmt.Println("== Limit 10,000, rule: shed")
	p = &Pipeline{limit: 10000, rule: Shed}
	for t := 1; t <= 60; t++ {
		p.Second()
	}
	fmt.Printf("t=60 s: queue %d, rows shed %d\n", p.queue, p.shed)

	fmt.Println("== Pull from a log on disk")
	l := &Log{}
	for t := 1; t <= 60; t++ {
		l.appended += indexerRate
		l.Pull(loaderRate)
	}
	fmt.Printf("t=60 s: consumed %d, lag on disk %d, memory %3.0f %%\n", l.consumed, l.appended-l.consumed, baseMem*100)

	fmt.Println("== A real Go channel")
	d, m := ChannelDemo(1000, 10)
	fmt.Printf("1000 rows through a buffer of 10: delivered %d, longest queue %d\n", d, m)
}
