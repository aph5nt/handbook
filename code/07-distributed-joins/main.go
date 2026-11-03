// Broadcast, shuffle and colocate on three nodes. The program counts the
// rows each plan sends over the network.
package main

import (
	"fmt"
	"hash/crc32"
)

const nodes = 3
const broadcastLimit = 1000000 // the planner copies a side to every node only below this

// Table is what the planner knows about one table.
type Table struct {
	name    string
	rows    int    // real rows
	est     int    // rows the statistics say
	key     string // bucket key
	buckets int
	group   string // colocation group
}

// hash is the same function on every node, for every table.
func hash(tx string) uint32 { return crc32.ChecksumIEEE([]byte(tx)) }

// bucketOf returns a bucket from 0 to buckets-1.
func bucketOf(tx string, buckets int) int { return int(hash(tx) % uint32(buckets)) }

// nodeOf places a bucket on a node, from 1 to 3.
func nodeOf(bucket, buckets int) int { return bucket*nodes/buckets + 1 }

// plan picks how to join big with other, the way the planner does.
func plan(big, other Table) string {
	if big.group != "" && big.group == other.group && big.key == other.key && big.buckets == other.buckets {
		return "COLOCATE" // matching buckets already sit on the same node
	}
	if other.est <= broadcastLimit {
		return "BROADCAST" // copy the small side to every node
	}
	return "SHUFFLE" // send both sides to the node their key hashes to
}

// broadcastSent is what a broadcast copies: every real row to every other node.
func broadcastSent(t Table) int { return t.rows * (nodes - 1) }

func broadcast() {
	fmt.Printf("broadcast limit %s rows\n", comma(broadcastLimit))
	swaps := Table{name: "swaps", rows: 3000000000, est: 3000000000}
	registry := Table{name: "dex registry", rows: 100, est: 100}
	fmt.Printf("swaps join dex registry: plan %s, rows sent %s\n", plan(swaps, registry), comma(broadcastSent(registry)))
	transfers := Table{name: "transfers", rows: 2000000000, est: 100} // stale statistics
	fmt.Printf("swaps join transfers, statistics say %d: plan %s, rows sent %s\n", transfers.est, plan(swaps, transfers), comma(broadcastSent(transfers)))
	transfers.est = transfers.rows // ANALYZE refreshes the statistics
	fmt.Printf("after ANALYZE: plan %s\n", plan(swaps, transfers))
}

// shuffle places rows by their own keys, then sends each to the node of hash(tx_hash).
func shuffle() {
	first, count := 0xa000, 3000
	sent := 0
	for k := first; k < first+count; k++ {
		tx := fmt.Sprintf("0x%04x", k)
		pool := k % 3                               // swaps are spread by pool
		block := (k - first) / 50                   // 50 transfers per block
		swapNode, transferNode := pool+1, block%3+1 // transfers are spread by block
		target := nodeOf(bucketOf(tx, 12), 12)
		if swapNode != target {
			sent++ // the swap crosses the network
		}
		if transferNode != target {
			sent++ // the transfer crosses the network
		}
		if tx == "0xab12" {
			fmt.Printf("shuffle: 0xab12 swap on node %d, transfer on node %d, hash sends both to node %d\n", swapNode, transferNode, target)
		}
	}
	fmt.Printf("shuffle: %s of %s rows sent (%d%%)\n", comma(sent), comma(2*count), 100*sent/(2*count))
}

func colocate() {
	swaps := Table{name: "swaps", est: 3000000000, key: "tx_hash", buckets: 12, group: "tx"}
	transfers := Table{name: "transfers", est: 2000000000, key: "tx_hash", buckets: 12, group: "tx"}
	b := bucketOf("0xab12", 12)
	fmt.Printf("colocate: 0xab12 in bucket %d of both tables, on node %d\n", b+1, nodeOf(b, 12))
	sent := 0
	for k := 0xa000; k < 0xa000+3000; k++ {
		tx := fmt.Sprintf("0x%04x", k)
		if nodeOf(bucketOf(tx, swaps.buckets), swaps.buckets) != nodeOf(bucketOf(tx, transfers.buckets), transfers.buckets) {
			sent++ // the two rows of tx sit on different nodes
		}
	}
	fmt.Printf("colocate: plan %s, rows sent %d\n", plan(swaps, transfers), sent)
	transfers.buckets = 16 // someone recreates transfers
	fmt.Printf("16 buckets: 0xab12 in transfers bucket %d, plan %s\n", bucketOf("0xab12", 16)+1, plan(swaps, transfers))
	transfers.buckets = 12 // fix: same key, same count, same group
	fmt.Printf("12 buckets again: plan %s\n", plan(swaps, transfers))
}

// comma writes 4000000000 as 4,000,000,000.
func comma(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func main() {
	broadcast()
	shuffle()
	colocate()
}
