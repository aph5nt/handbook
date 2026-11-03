// Command discord replays the hot channel lesson: a partition that puts one
// busy channel on one node, the GC stop that hits quiet channels, and request
// coalescing that turns 1,000 reads into one query. The limit is a model figure.
package main

import (
	"fmt"
	"hash/fnv"
)

const Limit = 500 // reads a node takes before its heap fills, a model figure

func NodeOf(channel string, bucket int) int {
	h := fnv.New32a()
	fmt.Fprintf(h, "%s/%d", channel, bucket)
	return int(h.Sum32()%3) + 1
}

type Node struct {
	Load    int
	Queries int
	NoGC    bool // ScyllaDB: no Java heap
	Stopped bool
}

func (n *Node) Query() {
	n.Load++
	n.Queries++
	if n.Load > Limit && !n.NoGC {
		n.Stopped = true
	}
}

type Service struct {
	InFlight map[string]int // key -> readers waiting
}

// Read returns true when this reader sends the query.
func (s *Service) Read(key string, n *Node) bool {
	if _, busy := s.InFlight[key]; busy {
		s.InFlight[key]++
		return false
	}
	s.InFlight[key] = 0
	n.Query()
	return true
}

func (s *Service) Answer(key string) int {
	waiting := s.InFlight[key]
	delete(s.InFlight, key)
	return waiting
}

// SwapOnly is a new database with no service in front.
func SwapOnly(n *Node, readers int) {
	for i := 0; i < readers; i++ {
		n.Query()
	}
}

func quietOn(node, want int) []string {
	var out []string
	for i := 0; len(out) < want; i++ {
		c := fmt.Sprintf("quiet-%d", i)
		if NodeOf(c, 2917) == node {
			out = append(out, c)
		}
	}
	return out
}

func main() {
	const big, bucket = "announcements", 2917
	home := NodeOf(big, bucket)
	fmt.Printf("before step 1: %s bucket %d lives on node %d\n", big, bucket, home)
	n := &Node{}
	for i := 0; i < 1000; i++ {
		n.Query()
	}
	fmt.Printf("before step 3-5: %d queries, limit %d, node stopped: %v\n", n.Queries, Limit, n.Stopped)
	fmt.Printf("before step 6: quiet channels on node %d that now wait: %d\n", home, len(quietOn(home, 200)))

	n2 := &Node{}
	s := &Service{InFlight: map[string]int{}}
	key := fmt.Sprintf("%s/%d", big, bucket)
	sent := 0
	for i := 0; i < 1000; i++ {
		if s.Read(key, n2) {
			sent++
		}
	}
	waited := s.Answer(key)
	fmt.Printf("after step 3-4: readers 1000, queries %d, waited %d, node stopped: %v\n", sent, waited, n2.Stopped)

	n3 := &Node{NoGC: true}
	SwapOnly(n3, 1000)
	fmt.Printf("after step 6: new database, no coalescing: %d queries on node %d, stopped: %v\n", n3.Queries, home, n3.Stopped)
}
