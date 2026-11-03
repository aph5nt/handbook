// Quorums: three replicas hold a token price. The program replays W=1 R=1 (a
// stale read), W=2 R=2 (the overlap and read repair), one node down, and the
// partition where replica 3 must refuse or answer stale.
package main

import "fmt"

// Replica holds one copy of the price, as a version number.
type Replica struct {
	id      int
	version int
	up      bool
}

// Cluster is N replicas, and the copies still in flight in the background.
type Cluster struct {
	reps     []*Replica
	inFlight map[int]int // replica id -> version on its way
}

// Overlap reports whether every read set meets every write set.
func Overlap(r, w, n int) bool { return r+w > n }

// Write sends v to all replicas. ackOrder is who answers first. It is done
// after W acks. The others keep the copy in flight.
func (c *Cluster) Write(v, w int, ackOrder []int) (bool, []int) {
	acks := []int{}
	for _, id := range ackOrder {
		rep := c.reps[id-1]
		if !rep.up {
			continue
		}
		if len(acks) == w {
			c.inFlight[id] = v // not waited for
			continue
		}
		rep.version = v
		acks = append(acks, id)
	}
	return len(acks) == w, acks
}

// Read asks the given R replicas, returns the newest version, and repairs any
// replica that answered older.
func (c *Cluster) Read(ask []int) (int, []int) {
	newest := 0
	for _, id := range ask {
		if v := c.reps[id-1].version; v > newest {
			newest = v
		}
	}
	repaired := []int{}
	for _, id := range ask {
		if c.reps[id-1].version < newest {
			c.reps[id-1].version = newest
			repaired = append(repaired, id)
		}
	}
	return newest, repaired
}

// ReadCut is a read on replica id when it reaches only reachable replicas.
func (c *Cluster) ReadCut(id, reachable int, consistent bool) (int, error) {
	if consistent && reachable <= len(c.reps)/2 {
		return 0, fmt.Errorf("no quorum: reaches %d of %d", reachable, len(c.reps))
	}
	return c.reps[id-1].version, nil
}

// AntiEntropy delivers every copy still in flight.
func (c *Cluster) AntiEntropy() {
	for id, v := range c.inFlight {
		c.reps[id-1].version = v
		delete(c.inFlight, id)
	}
}

func newCluster() *Cluster {
	return &Cluster{inFlight: map[int]int{}, reps: []*Replica{{1, 1, true}, {2, 1, true}, {3, 1, true}}}
}

func versions(c *Cluster) string {
	out := ""
	for _, r := range c.reps {
		s := fmt.Sprintf("v%d", r.version)
		if !r.up {
			s = "down"
		}
		out += fmt.Sprintf("replica %d: %s  ", r.id, s)
	}
	return out
}

func main() {
	fmt.Println("== W = 1, R = 1")
	c := newCluster()
	fmt.Println("R + W > N?", Overlap(1, 1, 3))
	ok, acks := c.Write(2, 1, []int{1, 2, 3})
	fmt.Println("write v2 done:", ok, "acks from", acks, "|", versions(c))
	v, _ := c.Read([]int{3})
	fmt.Printf("read asks replica 3: v%d, stale\n", v)
	c.AntiEntropy()
	fmt.Println("background copy arrives |", versions(c))

	fmt.Println("== W = 2, R = 2")
	c = newCluster()
	fmt.Println("R + W > N?", Overlap(2, 2, 3))
	ok, acks = c.Write(2, 2, []int{1, 2, 3})
	fmt.Println("write v2 done:", ok, "acks from", acks, "|", versions(c))
	c.inFlight = map[int]int{} // replica 3 is slow, its copy has not landed
	v, rep := c.Read([]int{2, 3})
	fmt.Printf("read asks 2 and 3: returns v%d, repaired %v | %s\n", v, rep, versions(c))
	c.reps[2].up = false
	ok, acks = c.Write(3, 2, []int{1, 2, 3})
	fmt.Println("replica 3 down, write v3 done:", ok, "acks from", acks)
	c.reps[1].up = false
	ok, _ = c.Write(4, 2, []int{1, 2, 3})
	fmt.Println("replicas 2 and 3 down, write v4 done:", ok)

	fmt.Println("== Partition: replica 3 alone")
	c = newCluster()
	ok, acks = c.Write(2, 2, []int{1, 2})
	fmt.Println("majority side write v2 done:", ok, "acks from", acks, "|", versions(c))
	_, err := c.ReadCut(3, 1, true)
	fmt.Println("CP read on replica 3:", err)
	v, _ = c.ReadCut(3, 1, false)
	fmt.Printf("AP read on replica 3: v%d, stale\n", v)
	c.inFlight[3] = 2
	c.AntiEntropy()
	fmt.Println("the split heals |", versions(c))
}
