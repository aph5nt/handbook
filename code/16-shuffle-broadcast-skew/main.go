// Shuffle, broadcast and skew: three nodes join swaps to pools, then count
// swaps per trader while one router address makes most of the swaps.
package main

import (
	"fmt"
	"hash/fnv"
)

const nodes = 3

type Swap struct {
	Trader string
	Pool   int
}

// owner picks the node that owns a key: a hash of the key, modulo the nodes.
func owner(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() % nodes)
}

// ShuffleJoin sends every swap and every pool to the node owning its pool id.
func ShuffleJoin(parts [][]Swap, pools map[int]string) (moved int) {
	for from, part := range parts {
		for _, s := range part {
			if owner(fmt.Sprint(s.Pool)) != from {
				moved++ // this swap crosses the network
			}
		}
	}
	return moved + len(pools) // the pool rows are hashed and sent too
}

// BroadcastJoin copies the small pool table to every node and joins locally.
func BroadcastJoin(parts [][]Swap, pools map[int]string) (moved, joined int) {
	moved = len(pools) * (nodes - 1) // node 1 already holds the pools
	for _, part := range parts {
		for _, s := range part {
			if _, ok := pools[s.Pool]; ok {
				joined++ // local lookup, no swap moves
			}
		}
	}
	return moved, joined
}

// GroupByTrader shuffles rows by trader and returns the rows each node holds.
// A salt splits one hot key into several keys before the hash.
func GroupByTrader(all []Swap, hot string, salt int) (load [nodes]int, partial map[string]int) {
	partial = map[string]int{}
	for k, s := range all {
		key := s.Trader
		if salt > 0 && key == hot {
			key = fmt.Sprintf("%s#%d", key, k%salt+1) // 0x7a06#1, #2 and #3 land on three nodes
		}
		load[owner(key)]++
		partial[key]++
	}
	return load, partial
}

// Combine adds the salted pieces back into one count per trader.
func Combine(partial map[string]int, hot string, salt int) int {
	n := partial[hot]
	for k := 1; k <= salt; k++ {
		n += partial[fmt.Sprintf("%s#%d", hot, k)]
	}
	return n
}

func main() {
	pools := map[int]string{1: "WETH/USDC", 2: "WETH/HOOD", 3: "USDC/HOOD", 4: "WETH/ARB"}
	traders := []string{"0xb2", "0x11", "0xf6", "0x88", "0xa1", "0xc3"} // two per node by hash
	router := "0x7a06"                                                  // a router contract, the hot key
	var all []Swap
	for k := 0; k < 600; k++ {
		t := traders[k%len(traders)]
		if k%10 < 8 {
			t = router // the router makes 8 of every 10 swaps
		}
		all = append(all, Swap{t, k%4 + 1})
	}
	parts := make([][]Swap, nodes)
	for k, s := range all {
		parts[k%nodes] = append(parts[k%nodes], s) // the swaps arrive spread evenly
	}
	fmt.Printf("swaps %d on %d nodes (%d each), pools %d\n", len(all), nodes, len(parts[0]), len(pools))
	fmt.Println("shuffle join: rows moved", ShuffleJoin(parts, pools))
	moved, joined := BroadcastJoin(parts, pools)
	fmt.Println("broadcast join: rows moved", moved, "rows joined", joined)

	load, partial := GroupByTrader(all, router, 0)
	fmt.Println("group by trader, no salt: rows per node", load, "router rows", partial[router])
	memory := 300 // rows one node can hold in this story
	for n, l := range load {
		if l > memory {
			fmt.Printf("node %d holds %d rows, limit %d: out of memory\n", n+1, l, memory)
		}
	}
	load, partial = GroupByTrader(all, router, 3)
	fmt.Println("group by trader, salt 3: rows per node", load)
	fmt.Println("router count after combine:", Combine(partial, router, 3))
}
