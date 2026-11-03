// Cluster sharding on three simulated nodes: a region hashes the id, asks the coordinator
// once per shard, forwards, and an entity starts on its first message. A dead node's
// shards move, and the entity rebuilds from the journal. Then the split brain replay.
package main

import (
	"fmt"
	"hash/crc32"
	"sort"
)

const shards = 10

func shardOf(entity string) int { return int(crc32.ChecksumIEEE([]byte(entity)) % shards) }

// Journal lives in the database, not on a node. It keeps device 42's history.
type Journal struct {
	snapshotInterval int
	after            []int // interval changes after the snapshot
}

// Coordinator is an actor: it owns the map from shard to node.
type Where struct {
	Shard int
	Reply chan int
}
type NodeDown struct{ Node int } // its shards now have no owner
type Rebalance struct {
	Dead  int
	Reply chan []string
}
type Coordinator struct {
	mailbox chan any
	owner   map[int]int
	asked   int
}

func newCoordinator(home map[int][]int) *Coordinator {
	c := &Coordinator{mailbox: make(chan any, 8), owner: map[int]int{}}
	for n, list := range home {
		for _, s := range list {
			c.owner[s] = n
		}
	}
	go func() {
		for m := range c.mailbox {
			switch v := m.(type) {
			case Where:
				c.asked++
				v.Reply <- c.owner[v.Shard]
			case NodeDown:
				for s, n := range c.owner {
					if n == v.Node {
						c.owner[s] = 0 // no owner yet
					}
				}
			case Rebalance:
				v.Reply <- c.rebalance(v.Dead)
			}
		}
	}()
	return c
}

// rebalance moves only the orphan shards. Highest shard first, to the node with the
// fewest shards. A tie goes to the higher node number.
func (c *Coordinator) rebalance(dead int) []string {
	count := map[int]int{}
	orphans := []int{}
	for s, n := range c.owner {
		if n == 0 {
			orphans = append(orphans, s)
		} else {
			count[n]++
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(orphans)))
	moves := []string{}
	for _, s := range orphans {
		best := 0
		for n := 1; n <= 3; n++ {
			if n == dead {
				continue
			}
			if best == 0 || count[n] <= count[best] {
				best = n
			}
		}
		c.owner[s] = best
		count[best]++
		moves = append(moves, fmt.Sprintf("shard %d -> node %d", s, best))
	}
	return moves
}

// Region is the actor on each node that every message enters.
type Region struct {
	node    int
	mailbox chan Envelope
	cache   map[int]int
	running map[string]bool
	coord   *Coordinator
	peers   map[int]*Region
	journal *Journal
	buffer  []Envelope // messages for a shard with no owner
	held    chan struct{}
}
type Envelope struct {
	Entity string
	Text   string
	Done   chan string
	Fwd    bool // forwarded by a peer: this node owns the shard
	Flush  bool // the shards have moved: send the buffer again
}

func (r *Region) run() {
	for e := range r.mailbox {
		if e.Flush {
			for _, b := range r.buffer {
				r.mailbox <- b
			}
			r.buffer = nil
			continue
		}
		s := shardOf(e.Entity)
		owner, known := r.cache[s]
		if e.Fwd {
			owner, known = r.node, true
		}
		if !known { // asked once per shard, never once per message
			reply := make(chan int)
			r.coord.mailbox <- Where{s, reply}
			owner = <-reply
			if owner == 0 { // no owner: hold it, lose nothing
				r.buffer = append(r.buffer, e)
				fmt.Printf("  node %d: shard %d has no owner, buffer %q\n", r.node, s, e.Text)
				r.held <- struct{}{}
				continue
			}
			r.cache[s] = owner
			fmt.Printf("  node %d: %s is shard %d, coordinator says node %d\n", r.node, e.Entity, s, owner)
		}
		if owner != r.node {
			e.Fwd = true
			r.peers[owner].mailbox <- e // forward
			continue
		}
		if !r.running[e.Entity] { // start on the first message
			r.running[e.Entity] = true
			if len(r.journal.after) > 0 {
				fmt.Printf("  node %d: %s rebuilt: snapshot + %d events\n", r.node, e.Entity, len(r.journal.after))
			} else {
				fmt.Printf("  node %d: %s started\n", r.node, e.Entity)
			}
		}
		e.Done <- fmt.Sprintf("  node %d: %s handled %q", r.node, e.Entity, e.Text)
	}
}

func send(r *Region, entity, text string) {
	done := make(chan string)
	r.mailbox <- Envelope{Entity: entity, Text: text, Done: done}
	fmt.Println(<-done)
}

func main() {
	home := map[int][]int{1: {0, 1, 2, 9}, 2: {3, 4, 7}, 3: {5, 6, 8}}
	journal := &Journal{}
	coord := newCoordinator(home)
	peers := map[int]*Region{}
	for n := 1; n <= 3; n++ {
		peers[n] = &Region{node: n, mailbox: make(chan Envelope, 8), cache: map[int]int{},
			running: map[string]bool{}, coord: coord, peers: peers, journal: journal, held: make(chan struct{}, 1)}
		go peers[n].run()
	}

	fmt.Println("== routing: a reading for device 42 reaches node 1")
	send(peers[1], "entity-42", "reading 21.5 C")
	send(peers[1], "entity-42", "reading 21.7 C")
	fmt.Printf("  the coordinator was asked %d time for 2 messages\n", coord.asked)

	fmt.Println("== node 2 dies: shards 3, 4 and 7 have no owner")
	journal.snapshotInterval, journal.after = 30, []int{60, 30}
	coord.mailbox <- NodeDown{2}
	delete(peers[1].cache, shardOf("entity-42")) // the region's answer is stale now
	done := make(chan string)
	peers[1].mailbox <- Envelope{Entity: "entity-42", Text: "reading 21.9 C", Done: done}
	<-peers[1].held // wait until the region has buffered it
	reply := make(chan []string)
	coord.mailbox <- Rebalance{2, reply}
	for _, m := range <-reply {
		fmt.Println("  coordinator:", m)
	}
	peers[1].mailbox <- Envelope{Flush: true}
	fmt.Println(<-done)

	fmt.Println("== network split: nodes 1 and 2 on side A, node 3 on side B, no resolver")
	owners := map[string][]int{"shard 7": {2, 3}}
	fmt.Printf("  side A keeps shard 7 on node 2, side B starts all 10 on node 3: owners %v\n", owners["shard 7"])
	forked := []string{"node 2: ConfigChanged 30 s", "node 3: ConfigChanged 60 s"}
	fmt.Printf("  journal for device 42: %v. Two writers, the history forks\n", forked)

	fmt.Println("== the resolver: keep majority, after a stable time")
	for _, side := range []struct {
		name  string
		nodes int
	}{{"side A", 2}, {"side B", 1}} {
		if side.nodes*2 > 3 { // strictly more than half of all nodes
			fmt.Printf("  %s: %d of 3 is a majority, stays\n", side.name, side.nodes)
		} else {
			fmt.Printf("  %s: %d of 3 is a minority, downs itself before it takes any shard\n", side.name, side.nodes)
		}
	}
	c2 := newCoordinator(home)
	reply = make(chan []string)
	c2.mailbox <- NodeDown{3}
	c2.mailbox <- Rebalance{3, reply}
	for _, m := range <-reply {
		fmt.Println("  coordinator:", m)
	}
}
