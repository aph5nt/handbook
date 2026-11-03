// False sharing: two counters in one 64-byte cache line slow each other down.
// The times vary by machine. The shape does not: shared is slowest.
package main

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

const writes = 20_000_000 // per worker

// Shared: a and b are 8 bytes each and sit in the same 64-byte line.
type Shared struct {
	a int64
	b int64
}

// Padded: 56 bytes after a push b into the next line.
type Padded struct {
	a int64
	_ [56]byte
	b int64
}

// run starts two workers on two cores. Each adds to its own counter.
func run(a, b *int64) time.Duration {
	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(2)
	for _, c := range []*int64{a, b} {
		go func(c *int64) {
			defer wg.Done()
			for i := 0; i < writes; i++ {
				atomic.AddInt64(c, 1) // each write needs the line in Modified state
			}
		}(c)
	}
	wg.Wait()
	return time.Since(start)
}

// local: each worker counts in a local variable and adds once at the end.
func local(total *int64) time.Duration {
	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(2)
	for w := 0; w < 2; w++ {
		go func() {
			defer wg.Done()
			n := int64(0)
			for i := 0; i < writes; i++ {
				n++ // a register, never shared
			}
			atomic.AddInt64(total, n) // one shared write per worker
		}()
	}
	wg.Wait()
	return time.Since(start)
}

func main() {
	fmt.Printf("cores: %d, writes per worker: %d\n", runtime.NumCPU(), writes)
	var s Shared
	var p Padded
	fmt.Printf("shared: a at byte %d, b at byte %d -> same 64-byte line: %v\n",
		unsafe.Offsetof(s.a), unsafe.Offsetof(s.b), unsafe.Offsetof(s.b)/64 == unsafe.Offsetof(s.a)/64)
	fmt.Printf("padded: a at byte %d, b at byte %d -> same 64-byte line: %v\n",
		unsafe.Offsetof(p.a), unsafe.Offsetof(p.b), unsafe.Offsetof(p.b)/64 == unsafe.Offsetof(p.a)/64)

	ts := run(&s.a, &s.b)
	tp := run(&p.a, &p.b)
	var total int64
	tl := local(&total)
	fmt.Printf("one line, two cores : %v  (a=%d b=%d)\n", ts.Round(time.Millisecond), s.a, s.b)
	fmt.Printf("padded, one line each: %v  (a=%d b=%d)\n", tp.Round(time.Millisecond), p.a, p.b)
	fmt.Printf("local then add once : %v  (total=%d)\n", tl.Round(time.Millisecond), total)
	fmt.Printf("shared is %.1fx slower than padded on this machine (times vary)\n", float64(ts)/float64(tp))
}
