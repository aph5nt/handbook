// Replication: the story of the lesson 12-replication.html, replayed. One
// balance on a primary and two replicas, copied with sync and with async
// replication, with a stale read, a slow disk and a failover that loses a write.
package main

import "fmt"

const localDisk = 5 // ms for a node to write to its own disk

// Node is one copy of the balance. Pos counts the writes it has applied.
type Node struct {
	Name    string
	Balance int
	Pos     int
	Disk    int // ms for a copy to arrive and reach this disk
}

// Change is one entry in the primary's log.
type Change struct{ Pos, Balance int }

// Cluster is a primary, its replicas and the log of changes not yet copied.
type Cluster struct {
	Primary  *Node
	Replicas []*Node
	Pending  []Change
}

func (n *Node) Apply(c Change) { n.Balance, n.Pos = c.Balance, c.Pos }

// WriteSync waits for every replica: as slow as the slowest copy.
func (cl *Cluster) WriteSync(balance int) (waitMs int) {
	c := Change{cl.Primary.Pos + 1, balance}
	cl.Primary.Apply(c)
	for _, r := range cl.Replicas {
		r.Apply(c)
		waitMs = max(waitMs, r.Disk)
	}
	return localDisk + waitMs
}

// WriteAsync answers after the local disk and copies later.
func (cl *Cluster) WriteAsync(balance int) (waitMs int) {
	c := Change{cl.Primary.Pos + 1, balance}
	cl.Primary.Apply(c)
	cl.Pending = append(cl.Pending, c)
	return localDisk
}

// CatchUp sends the pending changes to every replica.
func (cl *Cluster) CatchUp() {
	for _, c := range cl.Pending {
		for _, r := range cl.Replicas {
			r.Apply(c)
		}
	}
	cl.Pending = nil
}

// Read sends a user who wrote in the last 5 s to the primary, others to a replica.
func (cl *Cluster) Read(msSinceWrite int, readYourWrite bool) int {
	if readYourWrite && msSinceWrite < 5000 {
		return cl.Primary.Balance
	}
	return cl.Replicas[0].Balance
}

// Failover promotes the replica with the highest position. Writes past that
// position are lost.
func (cl *Cluster) Failover() (*Node, int) {
	best := cl.Replicas[0]
	for _, r := range cl.Replicas[1:] {
		if r.Pos > best.Pos {
			best = r
		}
	}
	return best, cl.Primary.Pos - best.Pos
}

func world() *Cluster {
	return &Cluster{
		Primary: &Node{Name: "primary", Balance: 100},
		Replicas: []*Node{
			{Name: "replica 1", Balance: 100, Disk: 10},
			{Name: "replica 2", Balance: 100, Disk: 12},
		},
	}
}

func main() {
	fmt.Println("== sync: wait for every copy")
	cl := world()
	ms := cl.WriteSync(90)
	fmt.Printf("1 pay 10: all copies %d %d %d, customer waits %d ms (5 + slowest 12)\n",
		cl.Primary.Balance, cl.Replicas[0].Balance, cl.Replicas[1].Balance, ms)
	fmt.Println("2 reload reads replica 1:", cl.Read(50, false))
	cl.Replicas[1].Disk = 2000
	ms = cl.WriteSync(80)
	fmt.Printf("3 replica 2 slow disk: next payment waits %d ms; a quorum of 2 of 3 would wait %d ms\n",
		ms, localDisk+cl.Replicas[0].Disk)

	fmt.Println("== async: the stale read and the lost write")
	cl = world()
	ms = cl.WriteAsync(90)
	fmt.Printf("1 pay 10: customer waits %d ms, primary %d, replicas %d %d\n",
		ms, cl.Primary.Balance, cl.Replicas[0].Balance, cl.Replicas[1].Balance)
	fmt.Println("2 reload 50 ms later reads replica 1:", cl.Read(50, false))
	fmt.Println("3 with read-your-own-write, within 5 s goes to the primary:", cl.Read(50, true))
	cl.CatchUp()
	fmt.Println("4 copies arrive, replica 1:", cl.Replicas[0].Balance, " replica 2:", cl.Replicas[1].Balance)
	cl.WriteAsync(80) // told ok, then the primary crashes before copying
	best, lost := cl.Failover()
	fmt.Printf("5 pay 10 again, crash before the copy: %s promoted with %d, positions %d vs %d, %d write lost\n",
		best.Name, best.Balance, best.Pos, cl.Primary.Pos, lost)
}
