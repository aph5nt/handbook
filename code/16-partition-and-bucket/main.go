// Partitioning and bucketing: a transfers table partitioned by day and
// bucketed by a hash of the sender. Count the storage pieces a query opens.
package main

import (
	"fmt"
	"hash/fnv"
)

const buckets = 4
const nodes = 3

var days = []string{"24 Sep", "25 Sep", "26 Sep"}

// bucketOf hashes the sender: the same key always lands in the same bucket.
func bucketOf(sender string) int {
	h := fnv.New32a()
	h.Write([]byte(sender))
	return int(h.Sum32()%buckets) + 1 // buckets are numbered 1 to 4
}

// nodeOf places bucket b of partition p on a backend node, evenly.
func nodeOf(p, b int) int { return (p+b-1)%nodes + 1 }

// Query is a filter on the partition key (day) and the bucket key (sender).
type Query struct{ Day, Sender string }

// Plan lists the (partition, bucket) pieces the engine must open.
func Plan(q Query) (pieces [][2]int) {
	for p, d := range days {
		if q.Day != "" && d != q.Day {
			continue // the day filter prunes this partition
		}
		for b := 1; b <= buckets; b++ {
			if q.Sender != "" && b != bucketOf(q.Sender) {
				continue // the hash says the sender is not here
			}
			pieces = append(pieces, [2]int{p, b})
		}
	}
	return pieces
}

// StarRocks DDL for the same layout
const layoutDDL = `PARTITION BY date_trunc('day', block_time)
DISTRIBUTED BY HASH(sender) BUCKETS 4`
const moveDDL = `ALTER TABLE transfers MODIFY PARTITION p20260924 SET ("storage_medium" = "HDD")`
const scopedAntiJoin = `WHERE t.day BETWEEN '2026-09-24' AND '2026-09-25'` // the anti-join, scoped

// The SQL of the "trap" tab, as the warehouse runs it.
const byAddressSQL = `-- the trap: partition key with millions of values
CREATE TABLE transfers_by_address (
  sender      VARCHAR(42),
  day         DATE,
  amount      DECIMAL(38, 18)
)
PARTITION BY LIST (sender) (...);   -- every address is a partition

-- a new address means a new partition in the metadata
ALTER TABLE transfers_by_address ADD PARTITION p_0x9f VALUES IN ('0x9f..');

-- a day question cannot prune: the key is sender
SELECT sum(amount) FROM transfers_by_address
WHERE  day = '2026-09-25';          -- opens all 224 partitions

-- the fix: time to prune, key to spread
CREATE TABLE transfers (
  sender      VARCHAR(42),
  day         DATE,
  amount      DECIMAL(38, 18)
)
PARTITION BY date_trunc('day', day)
DISTRIBUTED BY HASH(sender) BUCKETS 4;`

func main() {
	_ = byAddressSQL
	_, _, _ = layoutDDL, moveDDL, scopedAntiJoin
	total := len(days) * buckets
	sender := "0xabcd" // shown as 0xab.. in the lesson
	q := Query{Day: "25 Sep", Sender: sender}
	fmt.Printf("hash(0xab..) mod %d gives bucket %d\n", buckets, bucketOf(sender))
	fmt.Printf("day filter only: %d of %d pieces\n", len(Plan(Query{Day: q.Day})), total)
	p := Plan(q)
	fmt.Printf("day and sender: %d of %d pieces (partition %s, bucket %d, node %d)\n",
		len(p), total, days[p[0][0]], p[0][1], nodeOf(p[0][0], p[0][1]))
	// labels is bucketed by the same key, so its bucket 2 sits on the same node
	fmt.Printf("labels bucket %d on node %d: join is local\n", p[0][1], nodeOf(p[0][0], p[0][1]))
	// the failure: an anti-join over the whole range has no filter at all
	fmt.Printf("anti-join over the whole range: %d of %d pieces held in memory\n", len(Plan(Query{})), total)
	// the trap: partition by address. Every sender is its own partition.
	senders := 224
	fmt.Printf("partition by address: %d partitions, a day query opens %d of %d\n", senders, senders, senders)
}
