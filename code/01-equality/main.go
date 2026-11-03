// Equality follows from the kind: an entity compares by id, a value object by
// all its values. The hash uses the same fields, so sets and maps work.
package main

import "fmt"

// Pool is an entity: its balance changes, its id says who it is.
type Pool struct {
	ID      string
	Day     string
	Balance int
}

// Same is entity equality: the id only.
func (p Pool) Same(o Pool) bool { return p.ID == o.ID }

// SameByFields is failure 2: an entity compared by every field.
func (p Pool) SameByFields(o Pool) bool { return p == o }

// Money is a value object. Go structs compare by every field.
type Money struct {
	Amount   int
	Currency string
}

// BlockRange is a value object used as a map key: equal values, one entry.
type BlockRange struct{ From, To int }

// Indexed is a set of ranges already indexed.
type Indexed map[BlockRange]bool

// Add reports whether the range was new.
func (s Indexed) Add(r BlockRange) bool {
	if s[r] {
		return false // same hash, same fields: one entry
	}
	s[r] = true // first time
	return true
}

// RefSet is failure 1: compare by memory address, as a plain Java class does.
type RefSet map[*BlockRange]bool

// Add never finds an equal range, because every pointer is different.
func (s RefSet) Add(r *BlockRange) bool {
	if s[r] {
		return false
	}
	s[r] = true // a new pointer, every time
	return true
}

func main() {
	mon := Pool{"0xab", "Monday", 10}
	fri := Pool{"0xab", "Friday", 3}
	other := Pool{"0xcd", "Monday", 10}
	fmt.Println("same pool, two days:", mon.Same(fri))
	fmt.Println("two pools that look alike:", mon.Same(other))
	fmt.Println("Money 5 USD == 5 USD:", Money{5, "USD"} == Money{5, "USD"})
	fmt.Println("Money 5 USD == 5 EUR:", Money{5, "USD"} == Money{5, "EUR"})
	set := Indexed{}
	fmt.Println("add range:", set.Add(BlockRange{100, 200}), "- add again:", set.Add(BlockRange{100, 200}), "- entries", len(set))
	refs := RefSet{}
	fmt.Println("by reference:", refs.Add(&BlockRange{100, 200}), refs.Add(&BlockRange{100, 200}), "- entries", len(refs))
	fmt.Println("pool compared by fields:", mon.SameByFields(fri), "- two graph nodes")
}
