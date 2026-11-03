// Kafka partitions, keys and consumer groups, as in the lesson.
//
// partitionFor is Kafka's default partitioner: murmur2 of the key, made
// positive, mod the partition count. Go slices stand in for the partition
// logs. The assignor is a simple sticky one: each member keeps what it
// owned, up to its fair share, and the rest goes to members below their
// share. Single-threaded, so every number is exact.
package main

import (
	"fmt"
	"sort"
)

// murmur2 is the hash Kafka's Java client uses for keys.
func murmur2(data []byte) int32 {
	length := len(data)
	const seed uint32 = 0x9747b28c
	const m uint32 = 0x5bd1e995
	h := seed ^ uint32(length)
	for i := 0; i < length/4; i++ {
		k := uint32(data[i*4]) | uint32(data[i*4+1])<<8 | uint32(data[i*4+2])<<16 | uint32(data[i*4+3])<<24
		k *= m
		k ^= k >> 24
		k *= m
		h *= m
		h ^= k
	}
	switch length % 4 {
	case 3:
		h ^= uint32(data[(length&^3)+2]) << 16
		fallthrough
	case 2:
		h ^= uint32(data[(length&^3)+1]) << 8
		fallthrough
	case 1:
		h ^= uint32(data[length&^3])
		h *= m
	}
	h ^= h >> 13
	h *= m
	h ^= h >> 15
	return int32(h)
}

func partitionFor(key string, n int) int {
	return int(murmur2([]byte(key))&0x7fffffff) % n
}

// The wallets of the lesson, A to H, and the keys they publish with.
var key = map[string]string{"A": "0xA2", "B": "0xB4", "C": "0xC2", "D": "0xD1", "H": "0xH4"}

type Topic struct{ logs [3][]string }

func (t *Topic) Send(wallet, msg string) int {
	p := partitionFor(key[wallet], 3)
	t.logs[p] = append(t.logs[p], msg) // append-only, in arrival order
	return p
}

// assign is a sticky assignor: keep old partitions up to the fair share.
func assign(members []string, old map[string][]int, parts int) map[string][]int {
	sort.Strings(members)
	share := func(i int) int { // the first parts%n members get one extra
		s := parts / len(members)
		if i < parts%len(members) {
			s++
		}
		return s
	}
	out, taken := map[string][]int{}, map[int]bool{}
	for i, m := range members {
		for _, p := range old[m] {
			if len(out[m]) < share(i) {
				out[m] = append(out[m], p)
				taken[p] = true
			}
		}
	}
	for p := 0; p < parts; p++ {
		if taken[p] {
			continue
		}
		for i, m := range members {
			if len(out[m]) < share(i) {
				out[m] = append(out[m], p)
				break
			}
		}
	}
	return out
}

// process handles one transfer message.
func process(msg string) {
	fmt.Println("processed", msg)
}

func main() {
	fmt.Println("== Tab 1: normal ==")
	t := &Topic{}
	for _, w := range []string{"A", "A", "B", "C"} {
		fmt.Printf("wallet %s (key %s) -> P%d\n", w, key[w], partitionFor(key[w], 3))
	}
	t.Send("A", "A1")
	t.Send("A", "A2")
	t.Send("B", "B1")
	t.Send("C", "C1")
	own := assign([]string{"consumer 1", "consumer 2"}, nil, 3)
	fmt.Printf("step 3: group of two: %v\n", own)
	committed := [3]int{len(t.logs[0]), len(t.logs[1]), len(t.logs[2])}
	fmt.Printf("step 4: committed offsets P0=%d P1=%d P2=%d\n", committed[0], committed[1], committed[2])
	for _, m := range [][2]string{{"A", "A3"}, {"D", "D1"}, {"B", "B2"}, {"B", "B3"}, {"B", "B4"}} {
		t.Send(m[0], m[1])
	}
	lag := func(p int) int { return len(t.logs[p]) - committed[p] }
	// consume and commit
	consume := func(p int) {
		msg := t.logs[p][committed[p]] // read from the committed offset
		process(msg)
		committed[p]++ // a crash before this line: msg is read again
	}
	fmt.Printf("step 5: burst: lag P0=%d P1=%d P2=%d, consumer 1 owns %d of it\n", lag(0), lag(1), lag(2), lag(0)+lag(1))
	own = assign([]string{"consumer 1", "consumer 2", "consumer 3"}, own, 3)
	fmt.Printf("step 6: third consumer joins: %v\n", own)
	for p := range committed {
		committed[p] = len(t.logs[p]) // everyone catches up
	}
	t.Send("C", "C2")
	consume(2) // C2 processed and committed: offset 2
	t.Send("C", "C3")
	fmt.Printf("step 7: consumer 2 processed C3 but crashed before committing, P2 committed at %d\n", committed[2])
	own = assign([]string{"consumer 1", "consumer 3"}, own, 3)
	fmt.Printf("        rebalance: %v, P2 restarts at offset %d = %s (read twice)\n", own, committed[2], t.logs[2][committed[2]])
	fmt.Printf("step 8: P0 = %v, order per wallet held\n", t.logs[0])

	fmt.Println("== Tab 2: one hot key ==")
	const perTick, capacity = 30, 10 // 30 transfers a tick, a consumer handles 10 a tick
	run := func(split bool, ticks int) [3]int {
		var backlog [3]int
		seq := 0
		for tick := 0; tick < ticks; tick++ {
			for i := 0; i < perTick; i++ {
				var p int
				if i%10 != 9 { // 9 of every 10 transfers come from wallet H
					p = partitionFor(key["H"], 3)
					if split { // custom partitioner for the hot key only
						p = (p + seq) % 3
						seq++
					}
				} else {
					other := []string{"A", "B", "C"}[i%3]
					p = partitionFor(key[other], 3)
				}
				backlog[p]++
			}
			for p := range backlog {
				backlog[p] = max(0, backlog[p]-capacity)
			}
		}
		return backlog
	}
	fmt.Printf("step 2: wallet H (key %s) -> P%d\n", key["H"], partitionFor(key["H"], 3))
	fmt.Printf("step 3: after 10 ticks the backlog per partition is %v\n", run(false, 10))
	own = assign([]string{"c1", "c2", "c3", "c4"}, nil, 3)
	fmt.Println("step 4: a 4th consumer gets no partition: 3 partitions, 3 owners", own)
	fmt.Printf("step 5: hot key split by sequence: after 10 ticks the backlog is %v\n", run(true, 10))
}
