// Failure detection and failover: a Postgres-style leader with two
// followers. It replays the lesson: a safe failover (suspect, majority,
// fence, promote the most complete follower), then a timeout that is too
// short with no fence, which makes two leaders, then the fix.
package main

import "fmt"

// Node is a database node with its log position.
type Node struct {
	name     string
	pos      int  // highest log position it holds
	leader   bool // believes it is the leader
	fenced   bool // power cut or writes refused by storage
	lastSeen int  // second of the last heartbeat it received
	state    string
}

// Cluster holds the nodes and the detector settings.
type Cluster struct {
	old       *Node
	followers []*Node
	timeout   int  // seconds of silence before suspicion
	fence     bool // fence the old leader before promotion
}

// Heartbeat: the leader resets every follower's silence timer.
func (c *Cluster) Heartbeat(now int) {
	for _, f := range c.followers {
		f.lastSeen = now // silence goes back to 0
		f.state = "Healthy"
	}
}

// Tick runs on each follower: too much silence means Suspected.
func (c *Cluster) Tick(now int) {
	for _, f := range c.followers {
		if now-f.lastSeen > c.timeout {
			f.state = "Suspected"
		}
	}
}

// Decide counts the suspicions. A majority of the 3 nodes declares the
// leader dead, then the cluster fences it and promotes the best follower.
func (c *Cluster) Decide() *Node {
	votes := 0
	for _, f := range c.followers {
		if f.state == "Suspected" {
			votes++
		}
	}
	if votes < 2 { // 2 of 3 nodes is a majority
		return nil
	}
	if c.fence {
		c.old.fenced = true // STONITH: cut the old leader's power
	}
	best := c.followers[0]
	for _, f := range c.followers[1:] {
		if f.pos > best.pos {
			best = f // the highest log position loses no confirmed write
		}
	}
	best.leader = true
	return best
}

// Write: a node accepts a write only if it is a leader and not fenced.
func (n *Node) Write() bool {
	return n.leader && !n.fenced
}

func newCluster(timeout int, fence bool) *Cluster {
	return &Cluster{
		old:       &Node{name: "leader", pos: 120, leader: true},
		followers: []*Node{{name: "follower 1", pos: 120}, {name: "follower 2", pos: 118}},
		timeout:   timeout, fence: fence,
	}
}

func leaders(c *Cluster) int {
	n := 0
	for _, x := range append([]*Node{c.old}, c.followers...) {
		if x.Write() {
			n++
		}
	}
	return n
}

func main() {
	fmt.Println("== Safe failover: timeout 5 s, fence on")
	c := newCluster(5, true)
	c.Heartbeat(0)
	c.Tick(3)
	fmt.Printf("t=3s: silence 3 s <= 5 s: %s, %s\n", c.followers[0].state, c.followers[1].state)
	c.Tick(6)
	fmt.Printf("t=6s: silence 6 s > 5 s: %s, %s\n", c.followers[0].state, c.followers[1].state)
	nl := c.Decide()
	fmt.Printf("2 of 3 vote dead: old leader fenced=%v, promoted %s (pos %d, other has %d)\n", c.old.fenced, nl.name, nl.pos, c.followers[1].pos)
	fmt.Printf("nodes accepting writes: %d\n", leaders(c))

	fmt.Println("== Timeout too short: 1 s, no fence, leader pauses 3 s")
	c = newCluster(1, false)
	c.Heartbeat(0)
	c.Tick(2) // the leader is only paused
	nl = c.Decide()
	fmt.Printf("t=2s: promoted %s while the old leader was only paused\n", nl.name)
	fmt.Printf("old leader wakes at t=3s, still leader=%v: nodes accepting writes: %d (two leaders)\n", c.old.leader, leaders(c))

	fmt.Println("== Fix: same short timeout, fence on")
	c = newCluster(1, true)
	c.Heartbeat(0)
	c.Tick(2)
	nl = c.Decide()
	fmt.Printf("promoted %s, old leader fenced=%v: nodes accepting writes: %d\n", nl.name, c.old.fenced, leaders(c))
}
