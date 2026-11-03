// Head-of-line blocking: one stuck message holds a whole partition.
//
// A slice stands in for partition 3. m3 belongs to wallet 0x18 and its price
// lookup fails until the price service recovers. Three designs are replayed:
// retry in place, a retry topic with a blocked-key list, and lanes per key
// with a commit that cannot pass the lowest unfinished offset. Deterministic.
package main

import "fmt"

type Msg struct {
	Off    int
	ID     string
	Wallet string
}

var partition = []Msg{{1, "m1", "0x9f"}, {2, "m2", "0x51"}, {3, "m3", "0x18"}, {4, "m4", "0x77"},
	{5, "m5", "0xc2"}, {6, "m6", "0x18"}, {7, "m7", "0x9f"}, {8, "m8", "0x51"}}

func needsPrice(m Msg) bool { return m.ID == "m3" } // this price lookup times out

// inPlace: retry the head until it works. Nothing behind it moves.
func inPlace(priceUp bool) []string {
	var done []string
	for _, m := range partition {
		if needsPrice(m) && !priceUp {
			return done // stuck at m3, retrying
		}
		done = append(done, m.ID)
	}
	return done
}

// retryTopic moves a failing message aside and commits past it.
func retryTopic(blockKeys bool) (done, retry []string) {
	blocked := map[string]bool{}
	for _, m := range partition {
		if blockKeys && blocked[m.Wallet] {
			retry = append(retry, m.ID) // behind its own key's message
			continue
		}
		if needsPrice(m) {
			retry = append(retry, m.ID)
			blocked[m.Wallet] = true
			continue
		}
		done = append(done, m.ID)
	}
	if blockKeys {
		// retry consumer: m3, then m6, in order, then free the key
		delete(blocked, "0x18")
	}
	return
}

// lanes processes per wallet. The commit stops at the lowest unfinished offset.
func lanes() (done []string, commit int) {
	stuck := map[string]bool{}
	finished := map[int]bool{}
	for _, m := range partition {
		if needsPrice(m) || stuck[m.Wallet] {
			stuck[m.Wallet] = true // its lane waits, the others go on
			continue
		}
		done = append(done, m.ID)
		finished[m.Off] = true
	}
	commit = 1
	for finished[commit] {
		commit++
	}
	return
}

func main() {
	fmt.Println("== Tab 1: retry in place ==")
	fmt.Printf("price down: done %v, m4..m8 wait, none of them needs a price\n", inPlace(false))

	fmt.Println("== Tab 2: retry topic ==")
	done, retry := retryTopic(false)
	fmt.Printf("no key list: done %v, retry %v: m6 of 0x18 ran before m3 of 0x18\n", done, retry)
	done, retry = retryTopic(true)
	fmt.Printf("blocked keys: done %v, retry %v: 0x18 stays in order\n", done, retry)

	fmt.Println("== Tab 3: lanes ==")
	d, c := lanes()
	fmt.Printf("done %v, m3 and m6 wait in lane 0x18, commit stops at offset %d (m3)\n", d, c)
	fmt.Println("price back: m3 then m6 run, commit jumps to 9")
}
