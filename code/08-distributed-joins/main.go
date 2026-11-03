// An in-memory model of a four-node warehouse join. It needs no database:
// each row sits in a bucket by the hash of its bucket key, each bucket sits on
// a node, the planner rule picks colocate, bucket shuffle, broadcast or
// shuffle, and the program counts the rows that would cross the network.
// A row is taken as 100 bytes.
package main

import (
	"fmt"
	"hash/fnv"
)

const (
	nodes          = 4
	rowBytes       = 100
	broadcastLimit = 10 << 20 // 10 MB, Spark's default broadcast threshold
	swapsRows      = 40_000_000
)

// mix is splitmix64: it spreads every input bit over the whole output.
func mix(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

func hash(s string, k uint64) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return mix(h.Sum64() ^ mix(k))
}

// Table: rows are bucketed by Key ("pool" or the table's own id), and each
// bucket is placed on a node. Tables in one colocate group share Placement,
// so bucket b of both tables sits on the same node.
type Table struct {
	Name      string
	Rows      int
	KeyIsPool bool
	Buckets   int
	Placement string // the group name, or the table name when not grouped
}

func (t Table) bucketNode(b uint64) int { return int(hash("place:"+t.Placement, b) % nodes) }

func (t Table) rowNode(i, poolsN int) int {
	key := hash(t.Name, uint64(i)) // its own id
	if t.KeyIsPool {
		key = hash("pool", uint64(i%poolsN))
	}
	return t.bucketNode(key % uint64(t.Buckets))
}

// moved counts the rows of t whose node is not the node `owner` gives their pool.
func moved(t Table, poolsN int, owner func(pool uint64) int) int {
	n := 0
	for i := 0; i < t.Rows; i++ {
		if t.rowNode(i, poolsN) != owner(uint64(i%poolsN)) {
			n++
		}
	}
	return n
}

func planJoin(big, small Table, poolsN int) {
	switch {
	case big.KeyIsPool && small.KeyIsPool && big.Buckets == small.Buckets && big.Placement == small.Placement:
		fmt.Println("  colocate: same key, same bucket count, one group: 0 rows moved")
	case big.KeyIsPool:
		m := moved(small, poolsN, func(p uint64) int { return big.bucketNode(hash("pool", p) % uint64(big.Buckets)) })
		fmt.Printf("  bucket shuffle: pools rows moved %d = %.0f MB per query\n", m, float64(m*rowBytes)/1e6)
	case small.Rows*rowBytes <= broadcastLimit:
		fmt.Printf("  pools %d bytes <= 10 MB, broadcast: %d x %d nodes = %d bytes per query\n",
			small.Rows*rowBytes, small.Rows*rowBytes, nodes, small.Rows*rowBytes*nodes)
	default:
		fmt.Printf("  pools %d MB > 10 MB, no broadcast (a copy on every node would be %.1f GB)\n",
			small.Rows*rowBytes/1_000_000, float64(small.Rows*rowBytes*nodes)/1e9)
		owner := func(p uint64) int { return int(hash("shuffle", hash("pool", p)%16) % nodes) }
		ms, mp := moved(big, poolsN, owner), moved(small, poolsN, owner)
		fmt.Printf("  shuffle: swaps rows moved %d, pools rows moved %d, %.1f GB per query\n",
			ms, mp, float64((ms+mp)*rowBytes)/1e9)
	}
}

func main() {
	swapsByTx := Table{Name: "swaps", Rows: swapsRows, Buckets: 16, Placement: "swaps"}
	fmt.Println("Tab 1, pools small (1,000 rows), each table bucketed by its own id:")
	planJoin(swapsByTx, Table{Name: "pools", Rows: 1_000, Buckets: 16, Placement: "pools"}, 1_000)
	fmt.Println("Tab 1, pools grown to 4,000,000 rows:")
	planJoin(swapsByTx, Table{Name: "pools", Rows: 4_000_000, Buckets: 16, Placement: "pools"}, 4_000_000)

	swapsByPool := Table{Name: "swaps", Rows: swapsRows, KeyIsPool: true, Buckets: 16, Placement: "pool_group"}
	fmt.Println("Tab 2, both bucketed by pool, 16 buckets, group pool_group:")
	planJoin(swapsByPool, Table{Name: "pools", Rows: 4_000_000, KeyIsPool: true, Buckets: 16, Placement: "pool_group"}, 4_000_000)
	fmt.Println("Tab 2, pools rebuilt with 8 buckets, so it leaves the group:")
	planJoin(swapsByPool, Table{Name: "pools", Rows: 4_000_000, KeyIsPool: true, Buckets: 8, Placement: "pools"}, 4_000_000)
}
