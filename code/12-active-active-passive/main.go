// Replays active-active and active-passive: last write wins, a home region
// for each record, and a lease that fences off split brain.
package main

import "fmt"

// Write is one version of a row, with its clock time in milliseconds.
type Write struct {
	Value string
	At    int
}

// Region holds one copy of row 42.
type Region struct {
	Name string
	Row  Write
}

// WriteLocal is active-active with no owner: each region takes its own writes.
func WriteLocal(r *Region, w Write) {
	r.Row = w
}

// Replicate sends a region's row to the other, where a merge rule must pick one.
func Replicate(from, to *Region, merge func(a, b Write) Write) {
	to.Row = merge(to.Row, from.Row)
}

// LastWriteWins merges two versions: the later clock wins, the other is gone.
func LastWriteWins(a, b Write) Write {
	if b.At > a.At {
		return b
	}
	return a
}

// CapacityNeeded: with n regions, each must carry what a dead one leaves behind.
func CapacityNeeded(totalLoad, regions int) (perRegion, normalShare int) {
	perRegion = totalLoad / (regions - 1)
	return perRegion, 100 * totalLoad / regions / perRegion
}

// Home says which region may write a record. Other regions forward.
func Home(row int) string {
	if row%2 == 0 {
		return "region 1"
	}
	return "region 2"
}

// WriteRow applies the home rule.
func WriteRow(from *Region, home *Region, row int, w Write) string {
	if Home(row) != from.Name {
		home.Row = w
		return from.Name + " forwards to " + home.Name
	}
	from.Row = w
	return from.Name + " writes"
}

// Lease is the one fact of who is live, held in one place.
type Lease struct {
	Holder string
	Until  int
	Epoch  int
}

// Acquire is a compare-and-set: only one region can win it.
func (l *Lease) Acquire(region string, now, ttl int) bool {
	if l.Holder != region && now < l.Until {
		return false
	}
	if l.Holder != region {
		l.Epoch++
	}
	l.Holder, l.Until = region, now+ttl
	return true
}

// MayWrite is the fence: no lease, no write.
func (l *Lease) MayWrite(region string, now int) bool {
	return l.Holder == region && now < l.Until
}

func main() {
	per, share := CapacityNeeded(100, 2)
	fmt.Printf("two regions: each sized for %d percent of the load, runs at %d percent\n", per, share)
	hot := Write{"Kraken hot wallet", 1000}
	cold := Write{"Kraken cold wallet", 1400}
	r1, r2 := &Region{Name: "region 1"}, &Region{Name: "region 2"}
	WriteLocal(r1, hot)
	WriteLocal(r2, cold)
	Replicate(r1, r2, LastWriteWins)
	Replicate(r2, r1, LastWriteWins)
	fmt.Println("both wrote, then replicated: region 1", r1.Row.Value, ", region 2", r2.Row.Value, "; lost with no error:", hot.Value)
	fmt.Println("home of row 42:", Home(42))
	fmt.Println(WriteRow(r2, r1, 42, cold), "->", r1.Name, "holds", r1.Row.Value)
	fmt.Println(WriteRow(r1, r1, 42, hot), "->", r1.Name, "holds", r1.Row.Value)

	fmt.Println("\nactive-passive, ttl 60 s")
	l := &Lease{}
	fmt.Println("region 1 acquires at 0:", l.Acquire("region 1", 0, 60), "epoch", l.Epoch)
	fmt.Println("region 2 tries at 30, region 1 still holds:", l.Acquire("region 2", 30, 60))
	fmt.Println("link cut, region 1 cannot renew. region 2 tries at 61:", l.Acquire("region 2", 61, 60), "epoch", l.Epoch)
	fmt.Println("region 1 may write at 61:", l.MayWrite("region 1", 61))
	fmt.Println("region 2 may write at 61:", l.MayWrite("region 2", 61))
}
