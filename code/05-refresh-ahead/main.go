// Refresh-ahead: a read late in the TTL refreshes the key in the background,
// so a hot key never expires. 50 readers a second, TTL 60 s.
package main

import "fmt"

const ttl, zone = 60, 54 // seconds: a read older than zone (90 % of the TTL) refreshes

const readers = 50 // per second

type key struct {
	loadedAt   int
	refreshing bool
	refreshAt  int // when the background refresh answers
}

// read returns true when the reader had to wait for a load.
func read(k *key, now int, ahead bool, calls *int) bool {
	if k.refreshing && now >= k.refreshAt { // the background refresh answered
		k.loadedAt, k.refreshing = k.refreshAt, false
	}
	age := now - k.loadedAt
	if age >= ttl { // expired: this reader waits for a load
		*calls++
		k.loadedAt = now
		return true
	}
	if ahead && age > zone && !k.refreshing { // in the zone, nobody refreshing yet
		*calls++
		k.refreshing, k.refreshAt = true, now+1 // answer arrives in 1 s
	}
	return false // served from the cache at once
}

func simulate(ahead bool, until int) (calls, waited int) {
	k := &key{loadedAt: 0}
	for now := 1; now <= until; now++ {
		for r := 0; r < readers; r++ {
			if read(k, now, ahead, &calls) {
				waited++
				if !ahead {
					calls += readers - 1 // no coalescing: the rest of the second all miss too
					waited += readers - 1
					break
				}
			}
		}
	}
	return
}

func main() {
	c, w := simulate(false, 60)
	fmt.Printf("plain expiry, seconds 1-60: price service calls=%d, readers who waited=%d\n", c, w)
	c, w = simulate(true, 60)
	fmt.Printf("refresh-ahead, seconds 1-60: price service calls=%d, readers who waited=%d\n", c, w)
	c, w = simulate(true, 600)
	fmt.Printf("refresh-ahead, 10 minutes: calls=%d (about one a minute), waited=%d\n", c, w)

	// refresh on a timer versus refresh on a read
	const cached, read50 = 1000, 50
	fmt.Printf("\ntimer refreshes all %d keys each minute: %d calls a minute\n", cached, cached)
	fmt.Printf("refresh on read touches only the %d keys still read: %d calls a minute\n", read50, read50)

	allKeys := make([]string, 1000) // 1,000 keys, 50 still read
	timerCalls := 0
	refresh := func(k string) { timerCalls++ }
	for _, k := range allKeys { // wrong: a timer refreshes every key, read or not
		refresh(k)
	}
	fmt.Printf("a timer over every key: %d price service calls a round, for 50 keys still read\n", timerCalls)
}
