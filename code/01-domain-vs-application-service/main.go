// A rule about one aggregate lives in it. A rule across several lives in a
// domain service. An application service only loads, calls, saves, publishes.
package main

import (
	"errors"
	"fmt"
)

// ---- rich entity ----

// Wallet keeps its balance private. The rule lives next to the data.
type Wallet struct{ balance int }

func NewWallet(usd int) *Wallet { return &Wallet{balance: usd} }

// Charge is the only way to spend.
func (w *Wallet) Charge(n int) error {
	if n > w.balance {
		return errors.New("refused: not enough")
	}
	w.balance -= n
	return nil
}

// TestCharge needs no database: build, call, compare.
func TestCharge() bool {
	w := NewWallet(20)
	refused := w.Charge(30) != nil
	passed := w.Charge(15) == nil
	return refused && passed && w.balance == 5
}

// ---- domain service ----

// Labels, Flows and Prices are three aggregates.
type Labels struct{ NearMixer bool }
type Flows struct {
	HopsFromMixer int
	ToExchange    bool
}
type Prices struct{ Day string }

// Score is the answer, with its reasons.
type Score struct {
	Risk    string
	Reasons []string
}

// RiskScorer holds no state and changes nothing: same inputs, same score.
func RiskScorer(l Labels, f Flows, p Prices) Score {
	if l.NearMixer && f.ToExchange {
		return Score{"high", []string{fmt.Sprintf("%d hops from a mixer", f.HopsFromMixer), "went to an exchange"}}
	}
	return Score{Risk: "low"}
}

// TestScorer passes three objects in. Nothing is loaded and nothing is saved.
func TestScorer() bool {
	s := RiskScorer(Labels{true}, Flows{3, true}, Prices{"2026-10-01"})
	return s.Risk == "high"
}

// ---- application service ----

// Address is the aggregate this use case changes.
type Address struct {
	Hex   string
	L     Labels
	F     Flows
	Score Score
}

// Repo stores Addresses. Outbox rows commit in the same transaction.
type Repo struct {
	Rows   map[string]Address
	Outbox []string
}

func (r *Repo) Get(hex string) Address { return r.Rows[hex] }

// SaveWithEvent is one transaction: the address and the event together.
func (r *Repo) SaveWithEvent(a Address, ev string) {
	r.Rows[a.Hex] = a
	r.Outbox = append(r.Outbox, ev)
}

// ScreenAddress is a recipe. There is no if about risk in here.
func ScreenAddress(r *Repo, hex string, p Prices) Score {
	a := r.Get(hex)                       // 1. load
	a.Score = RiskScorer(a.L, a.F, p)     // 2. call the domain
	r.SaveWithEvent(a, "AddressScreened") // 3. save, 4. publish
	return a.Score
}

// ---- anemic model: the failure ----

// AnemicWallet has a public balance and a setter. It guards nothing.
type AnemicWallet struct{ Balance int }

func (w *AnemicWallet) SetBalance(n int) { w.Balance = n }

// Checkout copies the rule. Refund, Admin and Batch import copy it too.
func Checkout(w *AnemicWallet, n int) error {
	if n > w.Balance {
		return errors.New("refused: not enough")
	}
	w.SetBalance(w.Balance - n) // copy
	return nil
}

// Renewal was written last, and forgot the copy.
func Renewal(w *AnemicWallet, n int) { w.SetBalance(w.Balance - n) }

// The fix: the rule moves into Wallet.Charge, and every service calls it.
// The copied checks are deleted. There is no setter left to call.
func CheckoutFixed(w *Wallet, n int) error { return w.Charge(n) }
func RenewalFixed(w *Wallet, n int) error  { return w.Charge(n) }

func main() {
	w := NewWallet(20)
	fmt.Println("charge 30:", w.Charge(30))
	fmt.Println("charge 15:", w.Charge(15), "balance", w.balance)
	fmt.Println("unit test, no database:", TestCharge())
	l, f, p := Labels{true}, Flows{3, true}, Prices{"2026-10-01"}
	fmt.Printf("score: %+v\n", RiskScorer(l, f, p))
	r := &Repo{Rows: map[string]Address{"0xa": {Hex: "0xa", L: l, F: f}}}
	s := ScreenAddress(r, "0xa", p)
	fmt.Println("screened:", s.Risk, "stored:", r.Rows["0xa"].Score.Risk, "outbox:", r.Outbox)
	an := &AnemicWallet{Balance: 20}
	Renewal(an, 30)
	fmt.Println("anemic, Renewal forgot:", an.Balance)
	fixed := NewWallet(20)
	fmt.Println("rule in the entity, Renewal calls Charge:", RenewalFixed(fixed, 30), fixed.balance)
	fmt.Println("checkout through the entity:", CheckoutFixed(fixed, 15), fixed.balance)
	fmt.Println("scorer test, no database:", TestScorer())
}
