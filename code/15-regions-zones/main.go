// Zones and regions: a synchronous copy loses nothing when a zone dies. An
// asynchronous copy far away loses the writes that had not crossed yet.
// A spread rule keeps replicas out of one zone.
package main

import "fmt"

type DB struct {
	Primary   int // confirmed writes
	ReplicaB  int
	ReplicaC  int
	FarRegion int
}

// Write confirms only after the replicas in the other zones have it.
// The far region is copied without waiting.
func (d *DB) Write(farKeepsUp bool) {
	d.ReplicaB++ // a few milliseconds: the zones are close
	d.ReplicaC++
	d.Primary++ // confirmed to the client here
	if farKeepsUp {
		d.FarRegion++
	}
}

// place puts each replica in the zone with the fewest, if spread is on.
func place(replicas int, zones []string, spread bool) map[string]int {
	count := map[string]int{}
	for r := 0; r < replicas; r++ {
		best := zones[0]
		if spread {
			for _, z := range zones {
				if count[z] < count[best] {
					best = z
				}
			}
		}
		count[best]++
	}
	return count
}

func alive(p map[string]int, deadZone string) int {
	n := 0
	for z, c := range p {
		if z != deadZone {
			n += c
		}
	}
	return n
}

func main() {
	d := &DB{}
	for i := 0; i < 1000; i++ {
		d.Write(i < 997) // the last 3 writes have not crossed the ocean
	}
	fmt.Printf("confirmed %d, zone b %d, zone c %d, far region %d\n", d.Primary, d.ReplicaB, d.ReplicaC, d.FarRegion)
	fmt.Printf("zone a lost: promote zone b, writes lost %d\n", d.Primary-d.ReplicaB)
	fmt.Printf("region lost: promote the far copy, writes lost %d\n", d.Primary-d.FarRegion)

	zones := []string{"a", "b", "c"}
	for _, spread := range []bool{false, true} {
		p := place(3, zones, spread)
		fmt.Printf("spread rule %v: %v, zone a lost leaves %d of 3\n", spread, p, alive(p, "a"))
	}
}
