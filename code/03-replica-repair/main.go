// Repairing replicas: a price written while node 3 is down.
// Hinted handoff: node 4 keeps a hint and hands it over when node 3 returns.
// An expired hint leaves node 3 stale; read repair fixes a row that is read.
// Anti-entropy: two Merkle trees are compared from the root down, and only
// the one range that differs is copied.
package main

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

// Node is one replica: key -> version (1 for v1, 2 for v2).
type Node struct {
	name string
	up   bool
	data map[string]int
}

type hint struct {
	key     string
	version int
	expired bool
}

// Cluster: the key's replicas are nodes 1-3; node 4 is the stand-in.
type Cluster struct {
	replicas []*Node
	standIn  *Node
	hints    map[string][]hint // hints by the name of the node they belong to
}

// Write stores the version on every replica that is up, and a hint for the rest.
func (c *Cluster) Write(key string, version int) {
	for _, r := range c.replicas {
		if r.up {
			r.data[key] = version
		} else {
			c.hints[r.name] = append(c.hints[r.name], hint{key, version, false})
		}
	}
}

// Return brings a node back and hands over every hint that has not expired.
func (c *Cluster) Return(r *Node) {
	r.up = true
	for _, h := range c.hints[r.name] {
		if !h.expired {
			r.data[h.key] = h.version // hinted handoff
		}
	}
	delete(c.hints, r.name) // expired hints are simply lost
}

// Read asks two nodes, returns the newest version and repairs the older one.
func Read(key string, a, b *Node) int {
	newest := a.data[key]
	if b.data[key] > newest {
		newest = b.data[key]
	}
	for _, n := range []*Node{a, b} {
		if n.data[key] < newest {
			n.data[key] = newest // read repair
		}
	}
	return newest
}

// h is a short hash, three hex characters, as drawn in the lesson.
func h(s string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))[:3] }

// ranges splits a node's keys into 4 ranges by the first letter.
func ranges(n *Node) [4]string {
	var out [4][]string
	keys := make([]string, 0, len(n.data))
	for k := range n.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		i := int(k[0]-'a') * 4 / 26
		out[i] = append(out[i], fmt.Sprintf("%s=v%d", k, n.data[k]))
	}
	var r [4]string
	for i := range out {
		r[i] = strings.Join(out[i], ",")
	}
	return r
}

// Tree is a Merkle tree over 4 ranges: leaves, two middles, one root.
type Tree struct {
	leaf [4]string
	mid  [2]string
	root string
}

func Build(n *Node) Tree {
	var t Tree
	for i, r := range ranges(n) {
		t.leaf[i] = h(r) // leaf = hash of the rows of the range
	}
	t.mid[0], t.mid[1] = h(t.leaf[0]+t.leaf[1]), h(t.leaf[2]+t.leaf[3])
	t.root = h(t.mid[0] + t.mid[1]) // parent = hash of its children
	return t
}

// Repair walks down only where the hashes differ, and copies those ranges.
func Repair(src, dst *Node) (compared int, copied []int) {
	a, b := Build(src), Build(dst)
	compared++
	if a.root == b.root {
		return compared, nil // one comparison: equal data
	}
	for m := 0; m < 2; m++ {
		compared++
		if a.mid[m] == b.mid[m] {
			continue // equal below: skip both ranges
		}
		for l := 2 * m; l < 2*m+2; l++ {
			compared++
			if a.leaf[l] != b.leaf[l] {
				copied = append(copied, l+1)
				copyRange(src, dst, l)
			}
		}
	}
	return compared, copied
}

func copyRange(src, dst *Node, l int) {
	for k, v := range src.data {
		if int(k[0]-'a')*4/26 == l {
			dst.data[k] = v
		}
	}
}

func main() {
	n := func(name string) *Node {
		return &Node{name, true, map[string]int{"apple": 1, "fig": 1, "kiwi": 1, "pear": 1, "yam": 1}}
	}
	n1, n2, n3, n4 := n("node 1"), n("node 2"), n("node 3"), n("node 4")
	c := &Cluster{replicas: []*Node{n1, n2, n3}, standIn: n4, hints: map[string][]hint{}}

	fmt.Println("== Hints and read repair")
	n3.up = false
	c.Write("pear", 2)
	fmt.Printf("node 3 down, write pear v2: node1 v%d node2 v%d node3 v%d, hints for node 3: %d\n", n1.data["pear"], n2.data["pear"], n3.data["pear"], len(c.hints["node 3"]))
	c.Return(n3)
	fmt.Printf("node 3 returns, hinted handoff: node3 pear v%d\n", n3.data["pear"])

	n3.up = false
	c.Write("kiwi", 2)
	c.Write("yam", 2)
	for i := range c.hints["node 3"] {
		c.hints["node 3"][i].expired = true // down longer than the hint lifetime
	}
	c.Return(n3)
	fmt.Printf("long outage, hints expired: node3 kiwi v%d, yam v%d\n", n3.data["kiwi"], n3.data["yam"])
	got := Read("kiwi", n2, n3)
	fmt.Printf("read kiwi from nodes 2 and 3: returns v%d, node3 repaired to v%d\n", got, n3.data["kiwi"])
	fmt.Printf("yam is never read: node3 yam still v%d\n", n3.data["yam"])

	fmt.Println("== Anti-entropy with Merkle trees")
	a, b := Build(n1), Build(n3)
	fmt.Printf("node 1: root %s, mids %s %s, leaves %v\n", a.root, a.mid[0], a.mid[1], a.leaf)
	fmt.Printf("node 3: root %s, mids %s %s, leaves %v\n", b.root, b.mid[0], b.mid[1], b.leaf)
	compared, copied := Repair(n1, n3)
	fmt.Printf("repair: %d hash comparisons, copied ranges %v\n", compared, copied)
	b = Build(n3)
	fmt.Printf("after repair: node 3 root %s (node 1 root %s), yam v%d\n", b.root, a.root, n3.data["yam"])
}
