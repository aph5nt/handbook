// Command airbnb replays the Airbnb lesson: shadow traffic that compares the
// old code with a new service, the switch only at zero mismatches, and a block
// that hides twelve services behind one call. N is a model figure.
package main

import "fmt"

type Req struct {
	ID      int
	Special bool
}

func Monorail(r Req) int {
	if r.Special {
		return 90
	}
	return 100
}

type Service struct{ HandlesSpecial bool }

func (s Service) Price(r Req) int {
	if r.Special && s.HandlesSpecial {
		return 90
	}
	return 100
}

type Shadow struct {
	Svc                  Service
	Requests, Mismatches int
}

func (x *Shadow) Handle(r Req) int {
	old, cand := Monorail(r), x.Svc.Price(r)
	x.Requests++
	if old != cand {
		x.Mismatches++
	}
	return old
}

func (x *Shadow) CanSwitch(n int) bool {
	return x.Requests >= n && x.Mismatches == 0
}

func PageBefore(calls []int) (count, slowest int) {
	for _, ms := range calls {
		count++
		slowest = max(slowest, ms)
	}
	return
}

func BlockListing(id int, calls []int) int {
	_, slowest := PageBefore(calls)
	return slowest
}

func PageAfter(id int, calls []int) (count, ms int) {
	return 1, BlockListing(id, calls)
}

func run(x *Shadow, n int) (shown int) {
	for i := 0; i < n; i++ {
		r := Req{ID: i, Special: i%100 == 0}
		if x.Handle(r) == 90 {
			shown++
		}
	}
	return shown
}

func main() {
	x := &Shadow{}
	const n = 1000
	shown := run(x, n)
	fmt.Printf("split step 4-5: %d requests, %d mismatches, users saw %d correct special prices, can switch: %v\n", x.Requests, x.Mismatches, shown, x.CanSwitch(n))
	x = &Shadow{Svc: Service{HandlesSpecial: true}}
	run(x, n)
	fmt.Printf("split step 6: after the fix: %d requests, %d mismatches, can switch: %v\n", x.Requests, x.Mismatches, x.CanSwitch(n))

	calls := make([]int, 12)
	for i := range calls {
		calls[i] = 40
	}
	calls[7] = 900
	c, s := PageBefore(calls)
	fmt.Printf("blocks step 2: page makes %d calls, waits %d ms for the slowest\n", c, s)
	c, s = PageAfter(42, calls)
	fmt.Printf("blocks step 4: page makes %d call; the block still waits %d ms, and one team owns fixing it\n", c, s)
}
