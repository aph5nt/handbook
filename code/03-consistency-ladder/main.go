// Consistency ladder: a leader, a replica one log position behind, and a
// wallet page. It replays the lesson: the vanishing deposit (eventual read),
// the read-your-writes fix, and the username check that needs the top rung.
package main

import "fmt"

// Node is a copy of the data with the log position it has applied.
type Node struct {
	name string
	pos  int
	data map[string]int
}

func (n *Node) get(key string) (int, bool) { v, ok := n.data[key]; return v, ok }

// Cluster: one leader orders every write, one replica copies its log.
type Cluster struct {
	leader, replica *Node
	log             []entry // log[i] is position i+1
}

type entry struct {
	key   string
	value int
}

// Write goes to the leader. It returns the log position of the write.
func (c *Cluster) Write(key string, value int) int {
	c.log = append(c.log, entry{key, value})
	c.leader.pos = len(c.log)
	c.leader.data[key] = value
	return c.leader.pos // the client may keep this position
}

// Replicate applies the log entries that have reached the replica, up to pos.
func (c *Cluster) Replicate(upTo int) {
	for c.replica.pos < upTo {
		e := c.log[c.replica.pos]
		c.replica.data[e.key] = e.value
		c.replica.pos++
	}
}

// Read picks the replica, as the load balancer does. minPos 0 means
// "any copy will do" (eventual). minPos > 0 is read-your-writes.
func (c *Cluster) Read(key string, minPos int) (int, string) {
	r := c.replica
	if minPos == 0 {
		v, _ := r.get(key)
		return v, r.name // eventual: no check at all
	}
	if r.pos >= minPos {
		v, _ := r.get(key)
		return v, r.name // the replica is new enough
	}
	v, _ := c.leader.get(key)
	return v, c.leader.name // too old: go to the leader
}

// SignupStale checks a replica, then writes. Two people can both pass.
func (c *Cluster) SignupStale(name string) bool {
	if _, taken := c.replica.get("user:" + name); taken {
		return false
	}
	c.Write("user:"+name, 1)
	return true
}

// SignupLinearizable asks the leader to put the name only if absent.
func (c *Cluster) SignupLinearizable(name string) bool {
	if _, taken := c.leader.get("user:" + name); taken {
		return false
	}
	c.Write("user:"+name, 1)
	return true
}

func newCluster() *Cluster {
	c := &Cluster{leader: &Node{"leader", 0, map[string]int{}}, replica: &Node{"replica", 0, map[string]int{}}}
	for i := 0; i < 39; i++ { // history, so the balance write lands at position 40
		c.Write(fmt.Sprintf("other:%d", i), i)
	}
	c.Write("balance", 100) // position 40
	c.Replicate(40)
	return c
}

func main() {
	fmt.Println("== Eventual read: the deposit vanishes")
	c := newCluster()
	fmt.Printf("start: leader pos %d, replica pos %d, balance %d\n", c.leader.pos, c.replica.pos, c.leader.data["balance"])
	pos := c.Write("balance", 150)
	fmt.Printf("top up 50: leader stores it at pos %d, balance 150\n", pos)
	v, from := c.Read("balance", 0)
	fmt.Printf("refresh, minPos 0: %s answers %d (replica pos %d < %d): STALE\n", from, v, c.replica.pos, pos)
	c.Replicate(pos)
	v, from = c.Read("balance", 0)
	fmt.Printf("one second later: %s answers %d\n", from, v)

	fmt.Println("== Read-your-writes")
	c = newCluster()
	pos = c.Write("balance", 150)
	fmt.Printf("top up 50: ok, pos %d kept in the session\n", pos)
	v, from = c.Read("balance", pos)
	fmt.Printf("refresh, minPos %d: replica is at %d, so the %s answers %d\n", pos, c.replica.pos, from, v)

	fmt.Println("== Uniqueness on a stale replica")
	c = newCluster()
	a, b := c.SignupStale("kim"), c.SignupStale("kim")
	fmt.Printf("two signups for 'kim' checked on the replica: %v and %v: two owners\n", a, b)
	c = newCluster()
	a, b = c.SignupLinearizable("kim"), c.SignupLinearizable("kim")
	fmt.Printf("fix, put-if-absent on the leader: %v and %v: one owner\n", a, b)
}
