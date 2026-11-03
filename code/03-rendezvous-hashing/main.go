// Rendezvous hashing. It replays the lesson: eight wallets, five cache servers.
// Every client scores each server for a key and picks the highest. It then
// removes cache B and adds cache F, and counts what moves.
package main

import (
	"crypto/sha256"
	"fmt"
)

// seed is shared by every client, so all of them compute the same scores.
const seed = "248419"

// Score hashes the key and the server together into a number from 0 to 99.
// A strong hash matters: with a weak one, similar server names score alike.
func Score(key, server string) int {
	h := sha256.Sum256([]byte(key + "/" + server + "/" + seed))
	return (int(h[0])<<8 | int(h[1])) % 100
}

// Owner scores every server for the key and returns the highest.
func Owner(key string, servers []string) string {
	best, bestScore := "", -1
	for _, s := range servers { // one hash per server, per lookup
		if sc := Score(key, s); sc > bestScore {
			best, bestScore = s, sc
		}
	}
	return best
}

// Place returns the owner of every key.
func Place(keys, servers []string) map[string]string {
	owners := map[string]string{}
	for _, k := range keys {
		owners[k] = Owner(k, servers)
	}
	return owners
}

// Remove returns the server list without one server.
func Remove(servers []string, gone string) []string {
	var left []string
	for _, s := range servers {
		if s != gone {
			left = append(left, s)
		}
	}
	return left
}

// Split stores a hot key under n keys, so n servers share its traffic.
func Split(key string, n int, servers []string) []string {
	var owners []string
	for i := 1; i <= n; i++ {
		owners = append(owners, Owner(fmt.Sprintf("%s#%d", key, i), servers))
	}
	return owners
}

// Moved lists the keys whose owner changed.
func Moved(keys []string, before, after map[string]string) []string {
	var moved []string
	for _, k := range keys {
		if before[k] != after[k] {
			moved = append(moved, fmt.Sprintf("%s %s->%s", k, before[k], after[k]))
		}
	}
	return moved
}

func main() {
	keys := []string{"0x18", "0x9f", "0x2a", "0x4c", "0x51", "0x63", "0x77", "0x80"}
	five := []string{"A", "B", "C", "D", "E"}
	fmt.Println("== Scores, A B C D E F")
	for _, k := range keys {
		fmt.Print(k, ":")
		for _, s := range append(five[:5:5], "F") {
			fmt.Printf(" %2d", Score(k, s))
		}
		fmt.Println()
	}
	start := Place(keys, five)
	fmt.Println("== Owners with five servers")
	count := map[string]int{}
	for _, k := range keys {
		fmt.Printf("%s -> %s\n", k, start[k])
		count[start[k]]++
	}
	fmt.Printf("A %d keys, B %d, C %d, D %d, E %d\n", count["A"], count["B"], count["C"], count["D"], count["E"])

	noB := Remove(five, "B")
	afterB := Place(keys, noB)
	fmt.Println("== Cache B leaves:", Moved(keys, start, afterB))

	withF := append(noB[:len(noB):len(noB)], "F")
	afterF := Place(keys, withF)
	fmt.Println("== Cache F joins:", Moved(keys, afterB, afterF))
	fmt.Printf("moved since the start: %d of %d\n", len(Moved(keys, start, afterF)), len(keys))

	fmt.Println("== Compared with hash mod N, 5 to 4 servers")
	mod := 0
	for _, k := range keys {
		h := sha256.Sum256([]byte(k))
		n := int(h[0])<<8 | int(h[1])
		if n%5 != n%4 {
			mod++
		}
	}
	fmt.Printf("mod N moves %d of %d keys for one change\n", mod, len(keys))

	fmt.Println("== Hot key")
	fmt.Printf("every request for 0x18 goes to cache %s; split into 3 keys: caches %v\n", Owner("0x18", five), Split("0x18", 3, five))
}
