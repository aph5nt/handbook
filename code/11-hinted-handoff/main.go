// Command hinted replays the hinted handoff lesson: a stand-in holds a write
// for a rebooting node and hands it over, then a five-hour outage outlives
// the three-hour hint window and a Merkle tree finds the one range to copy.
package main

import (
	"fmt"
	"hash/fnv"
)

type Hint struct {
	Owner, Key, Value string
}

type Node struct {
	Name      string
	Up        bool
	DownSince int // hour
	Data      map[string]string
	Hints     []Hint
}

func Write(owners []*Node, standIn *Node, hour int, k, v string) (acks int) {
	for _, o := range owners {
		if o.Up {
			o.Data[k] = v
			acks++
		} else if hour-o.DownSince < 3 {
			standIn.Hints = append(standIn.Hints, Hint{o.Name, k, v})
		}
	}
	return acks
}

func (d *Node) Deliver(owner *Node) (sent int) {
	var keep []Hint
	for _, h := range d.Hints {
		if h.Owner == owner.Name && owner.Up {
			owner.Data[h.Key] = h.Value
			sent++
		} else {
			keep = append(keep, h)
		}
	}
	d.Hints = keep
	return sent
}

// Tree holds hashes: index 1 is the top, 2 and 3 its halves, 4..7 the leaves.
func Tree(n *Node) [8]uint32 {
	var t [8]uint32
	for i := 0; i < 4; i++ {
		t[4+i] = hashRange(n, i*25, i*25+24)
	}
	for i := 3; i >= 1; i-- {
		t[i] = t[2*i] ^ t[2*i+1]*16777619
	}
	return t
}

// Diff walks down from the top and returns the leaves that differ.
func Diff(a, b [8]uint32, i int, compared *int) []int {
	*compared++
	if a[i] == b[i] {
		return nil
	}
	if i >= 4 {
		return []int{i}
	}
	return append(Diff(a, b, 2*i, compared), Diff(a, b, 2*i+1, compared)...)
}

func hashRange(n *Node, from, to int) uint32 {
	h := fnv.New32a()
	for k := from; k <= to; k++ {
		if v, ok := n.Data[fmt.Sprintf("k%02d", k)]; ok {
			fmt.Fprintf(h, "%d=%s;", k, v)
		}
	}
	return h.Sum32()
}

func mk(name string) *Node { return &Node{Name: name, Up: true, Data: map[string]string{}} }

func main() {
	A, B, C, D := mk("A"), mk("B"), mk("C"), mk("D")
	owners := []*Node{B, A, C}
	fmt.Println("hand tab")
	C.Up, C.DownSince = false, 0
	fmt.Println("step 1: C is rebooting")
	acks := Write(owners, D, 0, "cart", "headphones")
	fmt.Printf("step 2-3: %d answers meet W = 2; D holds %d hint for %s\n", acks, len(D.Hints), D.Hints[0].Owner)
	fmt.Println("step 4: C still down, delivered:", D.Deliver(C))
	C.Up = true
	fmt.Println("step 5: C back after 20 minutes, delivered:", D.Deliver(C))
	fmt.Printf("step 6: C has %q, hints left %d\n", C.Data["cart"], len(D.Hints))

	fmt.Println("repair tab")
	A, B, C, D = mk("A"), mk("B"), mk("C"), mk("D")
	owners = []*Node{B, A, C}
	for k := 0; k < 100; k += 10 {
		Write(owners, D, 0, fmt.Sprintf("k%02d", k), "v1")
	}
	C.Up, C.DownSince = false, 10
	Write(owners, D, 11, "k10", "v2")
	Write(owners, D, 14, "k60", "v2")
	fmt.Printf("step 1: C down at hour 10 for 5 hours; write at hour 11 hinted (%d hint); write at hour 14 past the 3-hour window (%d hint)\n", len(D.Hints), len(D.Hints))
	C.Up = true
	D.Deliver(C)
	fmt.Printf("step 2: C back: k10 = %s (hint worked), k60 = %s (B has %s): stale\n", C.Data["k10"], C.Data["k60"], B.Data["k60"])
	tb, tc := Tree(B), Tree(C)
	fmt.Printf("step 3: top hashes %08x and %08x: differ\n", tb[1], tc[1])
	fmt.Printf("step 4: left half equal: %v, right half equal: %v\n", tb[2] == tc[2], tb[3] == tc[3])
	cmp := 0
	leaves := Diff(tb, tc, 1, &cmp)
	fmt.Printf("step 5: hashes compared %d, leaves that differ %v = keys %d to %d\n", cmp, leaves, (leaves[0]-4)*25, (leaves[0]-4)*25+24)
	C.Data["k60"] = B.Data["k60"]
	fmt.Println("step 6: copied that range; tops equal:", Tree(B)[1] == Tree(C)[1])
}
