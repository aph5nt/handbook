// Redundancy and N+1: the story of the lesson 12-redundancy-n-plus-one.html,
// replayed. The peak needs 3 copies, the team runs 4, then copies, a database
// and a whole zone fail, alone and together.
package main

import "fmt"

const n = 3 // copies the peak needs

// Copy is one service copy and the failure domains it sits in.
type Copy struct {
	Name, Zone, Rack string
	Up               bool
}

// Load is the share of the peak each healthy copy carries, in percent of one
// copy's capacity. With no database, no copy can serve anything.
func Load(cs []*Copy, dbUp bool) (int, int) {
	up := 0
	for _, c := range cs {
		if c.Up {
			up++
		}
	}
	if up == 0 || !dbUp {
		return up, -1 // nothing is served
	}
	return up, n * 100 / up
}

// FailDomain takes down every copy in one zone or one rack: they fail together.
func FailDomain(cs []*Copy, domain string) {
	for _, c := range cs {
		if c.Zone == domain || c.Rack == domain {
			c.Up = false
		}
	}
}

// Roll pushes a config to one copy first and stops at the first crash.
func Roll(cs []*Copy, crashes bool, oneFirst bool) int {
	hit := 0
	for _, c := range cs {
		hit++
		if crashes {
			c.Up = false
			if oneFirst {
				return hit // watched it crash: stop the roll
			}
		}
	}
	return hit
}

// PerZone is how many copies each zone needs so that losing any one zone
// still leaves N.
func PerZone(zones int) int {
	return (n + zones - 2) / (zones - 1) // ceil(n / (zones-1))
}

func world(zones ...string) []*Copy {
	cs := []*Copy{}
	for k, z := range zones {
		cs = append(cs, &Copy{Name: fmt.Sprint("copy ", k+1), Zone: z, Rack: fmt.Sprint(z, "/rack ", k/2), Up: true})
	}
	return cs
}

func say(step string, cs []*Copy, dbUp bool) {
	up, load := Load(cs, dbUp)
	if load < 0 {
		fmt.Printf("%s: %d copies up, nothing served\n", step, up)
		return
	}
	verdict := "ok"
	if load > 100 {
		verdict = "overloaded"
	}
	fmt.Printf("%s: %d copies up, each at %d%% - %s\n", step, up, load, verdict)
}

func main() {
	fmt.Println("== spread: N+1 across zones, N =", n)
	cs := world("zone 1", "zone 1", "zone 2", "zone 2")
	say("1 four copies", cs, true)
	cs[1].Up = false
	say("2 copy 2 crashes", cs, true)
	cs[2].Up = false
	say("3 copy 3 out for an upgrade too", cs, true)
	cs[1].Up, cs[2].Up = true, true
	say("4 the one database is down", cs, false)
	say("5 replica in zone 2 promoted", cs, true)
	FailDomain(cs, "zone 1") // the old primary was in zone 1 as well
	say("6 zone 1 loses power", cs, true)
	fmt.Printf("  to survive a zone: 3 zones of %d, or 2 zones of %d\n", PerZone(3), PerZone(2))

	fmt.Println("== corr: correlated failure")
	cs = world("zone 1", "zone 1", "zone 1", "zone 1")
	say("1 four copies, one zone, two racks", cs, true)
	FailDomain(cs, "zone 1")
	say("2 zone 1 loses power", cs, true)
	cs = world("zone 1", "zone 1", "zone 2", "zone 2")
	Roll(cs, true, false)
	say("3 bad config pushed to all at once", cs, true)
	cs = world("zone 1", "zone 1", "zone 2", "zone 2")
	got := Roll(cs, true, true)
	say(fmt.Sprintf("4 bad config to one copy first, roll stops after %d", got), cs, true)
	cs = world("zone 1", "zone 1", "zone 2", "zone 2")
	FailDomain(cs, "zone 1")
	say("5 drill: zone 1 off on purpose", cs, true)
}
