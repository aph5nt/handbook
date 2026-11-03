// Raft leader election: five nodes. The program replays a leader crash, the
// old leader coming back, a split vote, and a partition where the minority
// can never elect.
package main

import "fmt"

// Node is one Raft server. Only what an election needs.
type Node struct {
	id       string
	term     int
	role     string // follower, candidate, leader
	votedFor string
	up       bool
	votes    int
	logLen   int
}

// Cluster holds the nodes, and who can reach whom.
type Cluster struct {
	nodes map[string]*Node
	cut   map[string]bool // node id -> on the minority side of a split
}

func (c *Cluster) reach(a, b string) bool {
	return c.nodes[a].up && c.nodes[b].up && c.cut[a] == c.cut[b]
}

// Heartbeat: a follower accepts a term at least as high as its own. A leader
// that sees a higher term steps down.
func (c *Cluster) Heartbeat(leader, to string) {
	l, n := c.nodes[leader], c.nodes[to]
	if !c.reach(leader, to) || l.term < n.term {
		return
	}
	n.term, n.role = l.term, "follower"
}

// StartElection runs when a node's random timer fires.
func (c *Cluster) StartElection(id string) {
	n := c.nodes[id]
	n.term++
	n.role, n.votedFor, n.votes = "candidate", id, 1
}

// RequestVote: a node gives one vote per term, to a log at least as long.
func (c *Cluster) RequestVote(cand, voter string) bool {
	k, v := c.nodes[cand], c.nodes[voter]
	if !c.reach(cand, voter) || k.term < v.term {
		return false
	}
	if k.term > v.term { // a higher term: step down and forget the old vote
		v.term, v.role, v.votedFor = k.term, "follower", ""
	}
	if v.votedFor != "" || k.logLen < v.logLen {
		return false // one vote per term, and never for a shorter log
	}
	v.votedFor = cand
	k.votes++
	return true
}

// Tally: more than half of ALL nodes, dead or not, makes a leader.
func (c *Cluster) Tally(cand string) bool {
	k := c.nodes[cand]
	if k.votes > len(c.nodes)/2 {
		k.role = "leader"
		return true
	}
	return false
}

func newCluster(term int, leader string) *Cluster {
	c := &Cluster{nodes: map[string]*Node{}, cut: map[string]bool{}}
	for _, id := range []string{"n1", "n2", "n3", "n4", "n5"} {
		c.nodes[id] = &Node{id: id, term: term, role: "follower", up: true}
	}
	c.nodes[leader].role = "leader"
	return c
}

func (c *Cluster) election(cand string, voters ...string) {
	c.StartElection(cand)
	for _, v := range voters {
		c.RequestVote(cand, v)
	}
	n := c.nodes[cand]
	fmt.Printf("%s term %d: %d votes of %d, leader: %v\n", cand, n.term, n.votes, len(c.nodes), c.Tally(cand))
}

func main() {
	fmt.Println("== Leader dies, new leader")
	c := newCluster(3, "n2")
	c.nodes["n2"].up = false
	fmt.Println("n2 crashes, n5's timer fires first")
	c.election("n5", "n1", "n3", "n4")
	c.nodes["n2"].up = true
	fmt.Printf("n2 returns as %s in term %d\n", c.nodes["n2"].role, c.nodes["n2"].term)
	c.Heartbeat("n5", "n2")
	fmt.Printf("heartbeat term %d: n2 is now %s\n", c.nodes["n5"].term, c.nodes["n2"].role)

	fmt.Println("== Split vote")
	c = newCluster(3, "n2")
	c.nodes["n2"].up = false
	c.StartElection("n1")
	c.StartElection("n5")
	fmt.Println("n3 votes n1:", c.RequestVote("n1", "n3"), " n4 votes n5:", c.RequestVote("n5", "n4"), " n4 votes n1:", c.RequestVote("n1", "n4"))
	fmt.Printf("n1 %d votes, n5 %d votes, of 5: leader n1 %v, leader n5 %v\n", c.nodes["n1"].votes, c.nodes["n5"].votes, c.Tally("n1"), c.Tally("n5"))
	c.election("n5", "n1", "n3", "n4")

	fmt.Println("== Partition: n1 and n2 cut off from n3, n4 and leader n5")
	c = newCluster(5, "n5")
	c.cut["n1"], c.cut["n2"] = true, true
	for _, id := range []string{"n3", "n4", "n5"} {
		c.nodes[id].logLen = 10 // the majority side keeps committing
	}
	c.nodes["n1"].logLen, c.nodes["n2"].logLen = 8, 8
	c.election("n1", "n2")
	c.election("n1", "n2")
	fmt.Println("the split heals, n1 asks again at term 8")
	c.cut = map[string]bool{}
	c.election("n1", "n2", "n3", "n4", "n5")
	fmt.Printf("n5 saw a higher term and is now %s, term %d\n", c.nodes["n5"].role, c.nodes["n5"].term)
	c.election("n3", "n1", "n2", "n4", "n5")
}
