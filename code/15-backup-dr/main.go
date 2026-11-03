// RPO and RTO from one bad night's timeline. RPO is the gap back to the last
// good copy. RTO is the time until the service is back. A copy nobody has
// restored might not be good.
package main

import (
	"fmt"
	"time"
)

type Copy struct {
	At       time.Time
	Good     bool // the truth, which only a restore can tell
	Verified bool // a restore drill has proven it
}

// lastGood is what you learn at restore time.
func lastGood(copies []Copy) Copy {
	for i := len(copies) - 1; i >= 0; i-- {
		if copies[i].Good {
			return copies[i]
		}
	}
	return Copy{}
}

// drill restores each unverified copy into a scratch environment.
func drill(copies []Copy) int {
	bad := 0
	for i := range copies {
		copies[i].Verified = true
		if !copies[i].Good {
			bad++
		}
	}
	return bad
}

func at(day, hour int) time.Time {
	return time.Date(2026, 9, day, hour, 0, 0, 0, time.UTC)
}

func main() {
	good := []Copy{{At: at(9, 1), Good: true}, {At: at(10, 1), Good: true}} // 01:00, every night

	failure := at(10, 5) // 05:00: the disks fail
	c := lastGood(good)
	fmt.Printf("RPO = %v of data lost\n", failure.Sub(c.At))
	fmt.Printf("restore the %s copy\n", c.At.Format("Jan 2 15:04"))
	fmt.Printf("re-read the chain from %s to %s\n", c.At.Format("15:04"), failure.Format("15:04"))
	back := at(10, 7) // 07:00: back online
	fmt.Printf("RTO = %v of downtime\n", back.Sub(failure))

	var corrupt []Copy
	for d := 3; d <= 10; d++ {
		corrupt = append(corrupt, Copy{At: at(d, 1), Good: d == 3}) // silently bad from day 4
	}
	c = lastGood(corrupt)
	fmt.Printf("never restored: the newest good copy is %s, RPO = %v\n", c.At.Format("Jan 2 15:04"), failure.Sub(c.At))
	fmt.Printf("a monthly drill finds %d bad copies before the night you need one\n", drill(corrupt))
}
