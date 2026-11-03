// Hot spots: the story of the lesson 12-hot-spots.html, replayed. One exchange
// wallet, 0x4a, overloads one shard. Measure it, cache it, coalesce it, split
// it. Then adding nodes changes nothing, and time ranges make a hot today.
package main

import (
	"crypto/sha256"
	"fmt"
	"hash/fnv"
)

const (
	ordinary = 600  // reads per second, all other addresses together
	hotReads = 1200 // reads per second for 0x4a
)

// Shard is hash(key) mod n, numbered 1 to n: a remainder of 0 means shard n.
func Shard(key string, n int) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	if s := int(h.Sum32() % uint32(n)); s != 0 {
		return s
	}
	return n
}

// Reads spreads the ordinary load, then adds what really reaches the hot key's
// shard. Index 0 is unused so that load[2] is shard 2.
func Reads(n, reachShard int) []int {
	load := make([]int, n+1)
	for s := 1; s <= n; s++ {
		load[s] = ordinary / n
	}
	load[Shard("0x4a", n)] += reachShard
	return load
}

// Cache keeps one answer for a TTL.
type Cache struct {
	Expires int
	Queries int // queries that reached the shard
}

// Get answers from the cache, or asks the shard once and keeps the answer.
func (c *Cache) Get(now, ttl int) {
	if now < c.Expires {
		return // a hit: the shard is not asked
	}
	c.Queries++
	c.Expires = now + ttl
}

// Coalesce gives many identical reads in flight one shard query.
func Coalesce(asks int) (queries, answered int) {
	inFlight := false
	for k := 0; k < asks; k++ {
		if !inFlight {
			inFlight = true // the first asker runs the query
			queries++
		}
		answered++ // everyone shares its answer
	}
	return queries, answered
}

// SplitWrites hashes key#0 to key#parts-1 and counts writes per shard.
func SplitWrites(key string, parts, writes, n int) ([]int, map[int][]int) {
	load := make([]int, n+1)
	where := map[int][]int{}
	for p := 0; p < parts; p++ {
		s := Shard(fmt.Sprintf("%s#%d", key, p), n)
		load[s] += writes / parts
		where[s] = append(where[s], p)
	}
	return load, where
}

// TimeRange places a row by its age in days: 3 is today, 2 last week, 1 older.
func TimeRange(ageDays int) int {
	switch {
	case ageDays == 0:
		return 3 // every new row
	case ageDays <= 7:
		return 2
	default:
		return 1
	}
}

// PrefixGroup places a row by the first hex digit of its hash.
func PrefixGroup(row string) int {
	switch d := sha256.Sum256([]byte(row))[0] >> 4; {
	case d <= 4:
		return 1 // prefix 0-4
	case d <= 9:
		return 2 // prefix 5-9
	default:
		return 3 // prefix a-f
	}
}

func main() {
	fmt.Println("== fix: find and fix the hot key (reads per second, capacity 1000)")
	fmt.Println("1 even spread at 900 reads: 300 each")
	all := Reads(3, hotReads)
	fmt.Printf("2 0x4a is on shard %d: shards 1-3 %v\n", Shard("0x4a", 3), all[1:])
	fmt.Printf("3 per key: 0x4a takes %d of shard 2's %d reads, %d%%\n", hotReads, all[2], hotReads*100/all[2])
	c := &Cache{}
	for now := 0; now < 300; now++ {
		for r := 0; r < hotReads; r++ {
			c.Get(now, 30)
		}
	}
	fmt.Printf("4 cache 30 s: %d asks in 300 s, %d reach the shard: shards 1-3 %v\n", hotReads*300, c.Queries, Reads(3, 0)[1:])
	q, a := Coalesce(1000)
	fmt.Printf("5 cache expires, %d analysts ask at once: %d shard query\n", a, q)
	w, where := SplitWrites("0x4a", 10, 900, 3)
	fmt.Println("6 split 900 writes into 10 parts:", where)
	fmt.Printf("  writes %v, plus 200 reads each: shards 1-3 [%d %d %d]\n", w[1:], w[1]+200, w[2]+200, w[3]+200)

	fmt.Println("== trap: more nodes, and the time trap")
	fmt.Println("1 hot shard again:", Reads(3, hotReads)[1:])
	six := Reads(6, hotReads)
	fmt.Printf("2 six shards: 0x4a on shard %d, shards 1-6 %v\n", Shard("0x4a", 6), six[1:])
	r := make([]int, 4)
	for k := 0; k < 1300; k++ {
		r[TimeRange(0)]++ // a new transfer is always age 0
	}
	fmt.Println("3 range by time, last month, last week, today: rows", r[1:])
	g := make([]int, 4)
	for r := 0; r < 1300; r++ {
		g[PrefixGroup(fmt.Sprint("transfer-", r))]++
	}
	fmt.Println("4 hash prefix 0-4, 5-9, a-f: rows per group", g[1:])
}
