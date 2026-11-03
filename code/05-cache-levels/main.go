// Cache levels as a model: a read walks the levels, fastest first, and stops
// at the first hit. The times are orders of magnitude, not measurements.
package main

import "fmt"

type Level struct {
	name string
	ns   int // about how long an answer from this level takes
	keys map[string]bool
}

func levels() []*Level {
	ls := []*Level{{"CPU L1 cache", 1, nil}, {"CPU L2 cache", 4, nil},
		{"CPU L3 cache", 10, nil}, {"Main memory (RAM)", 100, nil},
		{"Process cache", 500, nil}, {"Redis", 500_000, nil},
		{"Database buffer pool", 800_000, nil}, {"Disk (SSD)", 5_000_000, nil}}
	for _, l := range ls {
		l.keys = map[string]bool{}
	}
	ls[len(ls)-1].keys["balance:4711"] = true // the disk always has it
	return ls
}

// Read walks down. A hit copies the key into every faster level.
func Read(ls []*Level, key string) (name string, ns, checked int) {
	for i, l := range ls {
		if l.keys[key] {
			for _, faster := range ls[:i] {
				faster.keys[key] = true // the next read stops sooner
			}
			return l.name, l.ns, i + 1
		}
	}
	return "nowhere", 0, len(ls)
}

func main() {
	const key = "balance:4711"
	for k := 0; k < 8; k++ {
		ls := levels()
		ls[k].keys[key] = true
		name, ns, checked := Read(ls, key)
		fmt.Printf("hit at %-24s about %9d ns, levels checked: %d\n", name, ns, checked)
	}

	ls := levels()
	name, ns, _ := Read(ls, key)
	fmt.Printf("\ncold start, first read: %s, about %d ns\n", name, ns)
	name, ns, _ = Read(ls, key)
	fmt.Printf("second read: %s, about %d ns\n", name, ns)

	const requests = 1000 // at once, on empty levels, before any fill is done
	fmt.Printf("%d requests at once on empty levels: %d ms of disk work, all at the slowest place\n",
		requests, requests*5_000_000/1_000_000)
}
