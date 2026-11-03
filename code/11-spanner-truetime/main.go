// Command spanner replays the TrueTime lesson with a simulated clock. Times
// are milliseconds of the same second: .006 in the lesson is 6 here.
package main

import "fmt"

type Clock struct {
	True  int // the real time, which nobody can read
	Doubt int // the width of the interval
	Skew  int // how far this clock runs behind, inside the interval
}

func (c *Clock) Now() (earliest, latest int) {
	return c.True - c.Skew, c.True - c.Skew + c.Doubt
}

// LoseGPS widens the interval. Commit wait below grows with it.
func (c *Clock) LoseGPS(doubt int) { c.Doubt = doubt }

type Row struct{ Stamp, Balance int }

// Commit returns the stamp and how long the leader waited.
func Commit(c *Clock, log *[]Row, balance int, skipWait bool) (stamp, waited int) {
	_, latest := c.Now()
	stamp = latest
	*log = append(*log, Row{stamp, balance})
	if skipWait {
		return stamp, 0
	}
	for {
		earliest, _ := c.Now()
		if earliest > stamp {
			return stamp, waited
		}
		c.True++
		waited++
	}
}

// Read stamps itself from the clock, then a copy answers, taking no locks.
func Read(c *Clock, log []Row) (readStamp, balance int) {
	_, readStamp = c.Now()
	for _, r := range log {
		if r.Stamp <= readStamp {
			balance = r.Balance
		}
	}
	return readStamp, balance
}

func main() {
	c := &Clock{Doubt: 6}
	log := []Row{{-1, 1000}}
	e, l := c.Now()
	fmt.Printf("step 2: now() = [.%03d, .%03d]\n", e, l)
	stamp, waited := Commit(c, &log, 900, false)
	fmt.Printf("step 3: stamp .%03d\nstep 4-5: commit wait %d ms, done at true time .%03d\n", stamp, waited, c.True)
	rs, bal := Read(c, log)
	fmt.Printf("step 6: Ben reads with stamp .%03d, balance %d, no locks\n", rs, bal)

	gps := &Clock{Doubt: 6}
	gps.LoseGPS(30)
	e, l = gps.Now()
	log2 := []Row{{-1, 1000}}
	stamp, waited = Commit(gps, &log2, 900, false)
	fmt.Printf("step 7: GPS lost: now() = [.%03d, .%03d], stamp .%03d, commit wait %d ms\n", e, l, stamp, waited)

	frankfurt := &Clock{Doubt: 6}
	log3 := []Row{{-1, 1000}}
	stamp, waited = Commit(frankfurt, &log3, 900, true)
	iowa := &Clock{True: 1, Doubt: 6, Skew: 5}
	rs, bal = Read(iowa, log3)
	fmt.Printf("no wait: done after %d ms with stamp .%03d. Iowa, 5 ms behind, reads later with stamp .%03d: balance %d, the transfer is missing\n", waited, stamp, rs, bal)
}
