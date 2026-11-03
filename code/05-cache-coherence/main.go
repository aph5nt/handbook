// Cache coherence: three instances cache one pool fee. Update messages
// can arrive out of order; versions fix that. Drop messages are safe in
// any order; a TTL bounds the damage of a lost one.
package main

import "fmt"

type copyOf struct {
	fee     float64
	version int
	has     bool
	expires int // seconds
}

type Instance struct {
	name         string
	c            copyOf
	checkVersion bool
}

// ApplyUpdate is update-on-write: the message carries the value.
func (in *Instance) ApplyUpdate(fee float64, version int) {
	if in.checkVersion && version <= in.c.version {
		return // older than what I hold: ignore it
	}
	in.c = copyOf{fee, version, true, 0}
}

// Drop is invalidate-on-write: forget, reload on the next read.
func (in *Instance) Drop() { in.c.has = false }

// Read reloads from the source on a miss or after the TTL.
func (in *Instance) Read(source float64, now, ttl int) float64 {
	if !in.c.has || (ttl > 0 && now >= in.c.expires) {
		in.c = copyOf{source, 0, true, now + ttl}
	}
	return in.c.fee
}

func fleet(checkVersion bool) []*Instance {
	var out []*Instance
	for _, n := range []string{"A", "B", "C"} {
		out = append(out, &Instance{name: n, c: copyOf{0.25, 1, true, 30}, checkVersion: checkVersion})
	}
	return out
}

func show(label string, f []*Instance) {
	fmt.Printf("  %-34s A=%.2f B=%.2f C=%.2f\n", label, f[0].c.fee, f[1].c.fee, f[2].c.fee)
}

func main() {
	for _, cv := range []bool{false, true} {
		fmt.Printf("== update on write, version check=%v ==\n", cv)
		f := fleet(cv)
		for _, in := range []*Instance{f[0], f[2]} { // A and C: in order
			in.ApplyUpdate(0.3, 2)
			in.ApplyUpdate(0.5, 3)
		}
		f[1].ApplyUpdate(0.5, 3) // B: version 3 arrives first
		f[1].ApplyUpdate(0.3, 2) // B: version 2 arrives late
		show("after v2 and v3, swapped for B:", f)
	}

	fmt.Println("== invalidate on write, TTL 30 s ==")
	f := fleet(false)
	source := 0.5 // the operator set 0.3, then 0.5
	for _, in := range f {
		in.Drop()
		in.Drop() // two drops in any order say the same thing
	}
	for _, in := range f {
		in.Read(source, 0, 30)
	}
	show("each copy reloads:", f)
	source = 0.6
	f[0].Drop()
	f[1].Drop() // the drop to C is lost
	for _, in := range f {
		in.Read(source, 10, 30)
	}
	show("at second 10, drop to C lost:", f)
	f[2].Read(source, 30, 30)
	show("at second 30, C's TTL expired:", f)
}
