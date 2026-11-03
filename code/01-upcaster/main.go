// Upcaster: old TransferRecorded events are upgraded when they are read.
// It replays the lesson: a v1 event stops a projector that knows only v3,
// then a chain of upcasters lets every event through. Stored events never change.
package main

import (
	"errors"
	"fmt"
)

// Stored is an event exactly as it sits in the store, with its shape version.
type Stored struct {
	Seq     int
	Version int
	Fields  map[string]string
}

// V3 is the only shape the projector understands.
type V3 struct {
	From, To, Amount, Token, Chain string
}

// upcasters[v] turns shape v into shape v+1. Pure: no lookups, no clock.
var upcasters = map[int]func(map[string]string) map[string]string{
	1: func(f map[string]string) map[string]string { // v1 only tracked ETH
		return with(f, "token", "ETH")
	},
	2: func(f map[string]string) map[string]string { // v2 events were all on robinhood
		return with(f, "chain", "robinhood")
	},
}

const newest = 3 // a new version raises this and adds one upcaster

// Read runs an event through the chain until it has the newest shape.
// It never writes the event back: the store is append only.
func Read(e Stored, upcast bool) (V3, error) {
	v, f := e.Version, e.Fields
	for upcast && v < newest {
		f = upcasters[v](f) // one small step: v to v+1
		v++
	}
	return Project(v, f)
}

// Project accepts only the newest shape.
func Project(v int, f map[string]string) (V3, error) {
	if v != newest || f["token"] == "" || f["chain"] == "" {
		return V3{}, errors.New("error: token missing, projection stops")
	}
	return V3{f["from"], f["to"], f["amount"], f["token"], f["chain"]}, nil
}

func with(f map[string]string, k, v string) map[string]string {
	out := map[string]string{k: v}
	for a, b := range f {
		out[a] = b // a copy: the stored event is never edited
	}
	return out
}

func store() []Stored {
	base := map[string]string{"from": "A", "to": "B", "amount": "10"}
	return []Stored{
		{1, 1, base},
		{2, 2, with(base, "token", "USDC")},
		{3, 3, with(with(base, "token", "USDC"), "chain", "robinhood")},
		{4, 1, base},
	}
}

func run(upcast bool) {
	for _, e := range store() {
		got, err := Read(e, upcast)
		if err != nil {
			fmt.Printf("  event %d (v%d): %v\n", e.Seq, e.Version, err)
			return
		}
		fmt.Printf("  event %d (v%d) read as v3: token %s, chain %s\n", e.Seq, e.Version, got.Token, got.Chain)
	}
}

func main() {
	fmt.Println("event 3 alone, no upcasters:")
	if got, err := Read(store()[2], false); err == nil {
		fmt.Printf("  event 3 (v3): token %s, chain %s\n", got.Token, got.Chain)
	}
	fmt.Println("all events, no upcasters:")
	run(false)
	fmt.Println("all events, upcaster chain v1->v2->v3:")
	run(true)
	fmt.Printf("stored event 1 still has %d fields: never edited\n", len(store()[0].Fields))
}
