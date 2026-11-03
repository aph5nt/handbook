// Distributed locks: two report workers and one file. Redlock takes a majority
// of five Redis nodes and gives no token, so a paused worker overwrites the
// file. An etcd lock gives a growing revision, and the file store refuses a
// stale one. Time is a plain number of seconds, so the run is exact.
package main

import "fmt"

// RedisNode holds one lock key with an expiry on its own clock.
type RedisNode struct {
	owner      string
	expiresAt  int
	clockAhead int // seconds this node's clock runs ahead
}

// SetNX is SET key value NX PX ttl: it succeeds only if the key is free.
func (n *RedisNode) SetNX(owner string, now, ttl int) bool {
	local := now + n.clockAhead
	if n.owner != "" && local < n.expiresAt {
		return false // the key still exists on this node's clock
	}
	n.owner, n.expiresAt = owner, local+ttl
	return true
}

// Redlock asks each node. A majority, 3 of 5, means the lock is held.
func Redlock(nodes []*RedisNode, reach []int, owner string, now, ttl int) bool {
	count := 0
	for _, i := range reach {
		if nodes[i].SetNX(owner, now, ttl) {
			count++
		}
	}
	return count >= 3
}

// PlainStore accepts every write. It cannot tell who is current.
type PlainStore struct{ file string }

// Write has no check at all.
func (s *PlainStore) Write(data string) bool {
	s.file = data
	return true
}

// Etcd keeps one lock key. Every change in the store raises the revision.
type Etcd struct {
	revision int
	holder   string
}

// Lock grants the free key and returns the revision: the fencing token.
func (e *Etcd) Lock(worker string) (int, bool) {
	if e.holder != "" {
		return 0, false
	}
	e.revision++
	e.holder = worker
	return e.revision, true
}

// LeaseExpired deletes the key of a holder that stopped renewing.
func (e *Etcd) LeaseExpired() {
	e.revision++ // the delete is a change too
	e.holder = ""
}

// FencedStore remembers the highest revision it has accepted.
type FencedStore struct {
	file    string
	highest int
}

// Write refuses a revision older than the highest seen.
func (s *FencedStore) Write(data string, rev int) bool {
	if rev < s.highest {
		return false // a stale holder stops here
	}
	s.highest = rev
	s.file = data
	return true
}

func newNodes() []*RedisNode {
	nodes := make([]*RedisNode, 5)
	for i := range nodes {
		nodes[i] = &RedisNode{}
	}
	return nodes
}

func main() {
	fmt.Println("== Redlock: two holders")
	nodes := newNodes()
	first := []int{0, 1, 2}  // worker 1 reaches nodes 1, 2, 3
	second := []int{2, 3, 4} // worker 2 reaches nodes 3, 4, 5
	fmt.Printf("t=0: worker 1 holds a majority: %v, lock lasts 10 s\n", Redlock(nodes, first, "worker 1", 0, 10))
	store := &PlainStore{file: "report v1"}
	fmt.Printf("t=10: lock expired, worker 2 holds a majority: %v\n", Redlock(nodes, second, "worker 2", 10, 10))
	store.Write("report v2")
	fmt.Printf("t=11: worker 2 writes, file is %q\n", store.file)
	ok := store.Write("report v1 by worker 1")
	fmt.Printf("t=15: worker 1 wakes and writes: accepted %v, file is %q\n", ok, store.file)

	nodes = newNodes()
	Redlock(nodes, first, "worker 1", 0, 10)
	nodes[2].clockAhead = 8 // node 3's clock jumps 8 s ahead
	fmt.Printf("clock jump, t=3: worker 1's lock runs until t=10, worker 2 holds a majority too: %v\n", Redlock(nodes, second, "worker 2", 3, 10))

	fmt.Println("== etcd lock with a fencing token")
	e := &Etcd{revision: 811}
	fenced := &FencedStore{file: "report v1"}
	rev1, _ := e.Lock("worker 1")
	fmt.Printf("worker 1 gets revision %d, write accepted: %v\n", rev1, fenced.Write("worker 1 first run", rev1))
	e.LeaseExpired()
	e.revision++ // some other key in etcd changes
	rev2, _ := e.Lock("worker 2")
	fmt.Printf("lease expired, worker 2 gets revision %d, write accepted: %v\n", rev2, fenced.Write("report v2", rev2))
	fmt.Printf("worker 1 wakes, writes with %d: accepted %v, file is %q\n", rev1, fenced.Write("stale", rev1), fenced.file)
}
