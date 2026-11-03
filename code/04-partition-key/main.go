// Partition key: which key keeps which order, and what adding a partition does.
//
// partitionFor is Kafka's default partitioner (murmur2 of the key, mod the
// partition count). Go slices stand in for partition logs. Deterministic.
package main

import "fmt"

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

// logs are the topic's partitions: partition -> messages in order.
var logs = map[int][]string{}

// sticky is the partition the sticky partitioner fills next.
var sticky int

// nextSticky spreads messages that have no key. In this example it moves on each call.
func nextSticky() int {
	p := sticky
	sticky = (sticky + 1) % 3
	return p
}

func send(key, msg string) {
	var p int
	if key != "" {
		p = partitionFor(key, 3) // 0x17 -> P1, every time
	} else {
		p = nextSticky() // no key: spread, no order
	}
	logs[p] = append(logs[p], msg)
}

// handle is the work of the consumer that owns P1.
func handle(m string) {
	fmt.Println("P1 owner handles", m)
}

// drain lets a partition's reader finish everything already in it.
func drain(msgs []string) {
	fmt.Printf("drained %d old messages first\n", len(msgs))
}

func main() {
	fmt.Println("== Tab 1: same key, same partition ==")
	for _, w := range []string{"0x17", "0x17", "0x77", "0xc2"} {
		fmt.Printf("key %s -> hash mod 3 = P%d\n", w, partitionFor(w, 3))
	}
	send("0x17", "0x17 deposit")
	send("0x17", "0x17 withdrawal")
	send("", "no-key ping")
	// the owner of P1 reads it in order
	for _, m := range logs[1] {
		handle(m)
	}

	fmt.Println("== Tab 2: choosing the key ==")
	// Key by transaction hash: two transfers of wallet 0x17 with different hashes.
	var tx1, tx2 string
	for i := 0; tx1 == "" || tx2 == ""; i++ {
		h := fmt.Sprintf("0xtx%04d", i)
		if p := partitionFor(h, 3); p == 0 && tx1 == "" {
			tx1 = h
		} else if p == 2 && tx2 == "" && tx1 != "" {
			tx2 = h
		}
	}
	pd := partitionFor(tx1, 3) // P0
	pw := partitionFor(tx2, 3) // P2: read by another consumer
	fmt.Printf("key = tx hash: deposit %s -> P%d, withdrawal %s -> P%d: order per wallet lost\n",
		tx1, pd, tx2, pw)

	// Key by token: 100 transfers, 60 of them WETH.
	var byToken [3]int
	tok := func(i int) string {
		switch i % 5 {
		case 3:
			return "LINK"
		case 4:
			return "UNI"
		}
		return "WETH"
	}
	for i := 0; i < 100; i++ {
		byToken[partitionFor(tok(i), 3)]++ // P0=20 P1=60 P2=20
	}
	fmt.Printf("key = token: per 100 transfers P0=%d P1=%d P2=%d (WETH is 60)\n", byToken[0], byToken[1], byToken[2])

	// Key by wallet: 3,000 wallets, one transfer each.
	var byWallet [3]int
	for i := 0; i < 3000; i++ {
		byWallet[partitionFor(fmt.Sprintf("0x%04x", i), 3)]++
	}
	fmt.Printf("key = wallet: 3,000 wallets P0=%d P1=%d P2=%d\n", byWallet[0], byWallet[1], byWallet[2])

	// Busiest key share: one market maker sends 600 of 3,600 transfers.
	top, total := 600, 3600
	share := float64(top) / float64(total)
	fmt.Printf("busiest key: %d of %d = %.0f%% of traffic, all on one partition\n", top, total, 100*share)

	fmt.Println("== Tab 3: adding a partition ==")
	logs := map[int][]string{}
	old := partitionFor("0x17", 3) // 1
	logs[old] = append(logs[old], "0x17 old", "0x17 old")
	now := partitionFor("0x17", 4) // 2: the modulus changed
	logs[now] = append(logs[now], "0x17 new")
	fmt.Printf("3 partitions: 0x17 -> P%d. 4 partitions: 0x17 -> P%d\n", old, now)
	fmt.Printf("P%d still holds %v while P%d holds %v: two readers, no order between them\n", old, logs[old], now, logs[now])
	first := logs[now][0] // P2's reader is free: "0x17 new" runs first
	fmt.Println("runs first:", first)
	// fix: drain the old partitions first, then raise the count
	drain(logs[old])
	members, parts := 5, 4
	idle := members - parts // 5 consumers, 4 partitions: 1
	fmt.Printf("%d consumers, %d partitions: %d idle\n", members, parts, idle)
}
