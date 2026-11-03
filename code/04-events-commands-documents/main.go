// Events, commands and documents: what a message asks of its reader.
//
// Go types make the three shapes explicit. Maps stand in for readers' stores,
// the indexer API and object storage. A broker's redelivery is a second call
// to the handler. Deterministic.
package main

import (
	"errors"
	"fmt"
)

type Command struct{ Name, To string } // an order, to one known reader
type Event struct {
	Name  string
	Block int
}                                     // a fact, past tense, any reader
type Document struct{ Rows []string } // data, no intent

// ---- an event named like a command ----

func addDelta(balance, delta int) int { return balance + delta } // UpdateBalance +50

// ---- notification versus carried state ----

type State struct{ Balance, Nonce int }

var apiUp = true

func api(addr string) (State, error) {
	if !apiUp {
		return State{}, errors.New("indexer API: connection refused")
	}
	return State{5, 12}, nil
}

// onNotify must call back to build the row.
func onNotify(addr string) (State, error) { return api(addr) }

// onCarried writes what the event already holds.
func onCarried(s State) State { return s }

func main() {
	fmt.Println("== Tab 1: three shapes ==")
	fmt.Printf("%+v\n%+v\n%+v\n", Command{"RebuildLayer", "graph sync"}, Event{"BlockIndexed", 9506}, Document{[]string{"row 1", "row 2"}})
	cache, ledger := 100, 100
	cache = addDelta(cache, 50)
	ledger = addDelta(ledger, 50)
	cache = addDelta(cache, 50) // the broker redelivers to the cache only
	fmt.Printf("UpdateBalance +50, redelivered once: cache %d, ledger %d: they disagree\n", cache, ledger)
	newBalance := 150
	cache, ledger = newBalance, newBalance
	cache = newBalance // the same redelivery, now harmless
	fmt.Printf("BalanceUpdated new balance 150, redelivered once: cache %d, ledger %d\n", cache, ledger)

	fmt.Println("== Tab 2: thin or full ==")
	s, err := onNotify("X")
	fmt.Printf("notification, API up: %+v err=%v\n", s, err)
	apiUp = false
	_, err = onNotify("X")
	fmt.Printf("notification, API down: row not built: %v\n", err)
	fmt.Printf("carried state, API down: %+v written\n", onCarried(State{5, 12}))
	_, err = onNotify("X") // balance only: still needs the nonce
	fmt.Printf("half the state, API down: still calls back: %v\n", err)

	fmt.Println("== Tab 3: too big for a message ==")
	blob := make([]byte, 5_000_000)
	store := map[string][]byte{"receipts/9506.bin": blob}
	msg := "receipts/9506.bin" // the claim check: a path, not the blob
	fmt.Printf("message %d bytes, blob %d bytes\n", len(msg), len(store[msg]))
	data := store[msg]
	fmt.Printf("reader A fetched %d bytes\n", len(data))
	delete(store, msg) // cleanup after reader A: too early
	if _, ok := store[msg]; !ok {
		fmt.Println("reader B follows the path: not found")
	}
	retentionDays, blobDays := 7, 8
	keep := blobDays > retentionDays // 8 > 7
	fmt.Printf("fix: keep the blob %d days, the message lives %d: blob outlives it: %v\n", blobDays, retentionDays, keep)
}
