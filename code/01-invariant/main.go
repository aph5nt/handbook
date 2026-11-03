// Invariant: a rule that must hold after every change. The root computes the
// new state, checks the rule, and only then saves.
package main

import (
	"errors"
	"fmt"
)

// Wallet is the aggregate. balance is private: no door skips the rule.
type Wallet struct{ balance int }

var errNegative = errors.New("refused: balance would be below zero")

// apply is the one path every method takes: compute, check, then save.
func (w *Wallet) apply(next int) error {
	if next < 0 { // the invariant
		return errNegative // nothing changed
	}
	w.balance = next
	return nil
}

// TopUp and Charge both end in the same check.
func (w *Wallet) TopUp(n int) error  { return w.apply(w.balance + n) }
func (w *Wallet) Charge(n int) error { return w.apply(w.balance - n) }

// LooseWallet is the failure: the rule lives in one caller, the setter is public.
type LooseWallet struct{ Balance int }

func (w *LooseWallet) SetBalance(n int) { w.Balance = n }

// Controller is the only place that checks. A batch import never comes here.
func Controller(w *LooseWallet, charge int) error {
	if w.Balance-charge < 0 {
		return errNegative
	}
	w.SetBalance(w.Balance - charge)
	return nil
}

// SumRule is a rule across two aggregates: each sees only its own stale read.
func SumRule(own, seenOther, topUp, limit int) bool { return own+topUp+seenOther < limit }

func main() {
	w := &Wallet{balance: 20}
	fmt.Println("topUp 10:", w.TopUp(10), "balance", w.balance)
	fmt.Println("charge 45:", w.Charge(45), "balance", w.balance)
	fmt.Println("charge 25:", w.Charge(25), "balance", w.balance)
	lw := &LooseWallet{Balance: 5}
	lw.SetBalance(-10) // the batch import
	fmt.Println("batch import, stored balance:", lw.Balance)
	a, b := 500, 0
	okA := SumRule(a, b, 400, 1000) // A sees B as 0
	okB := SumRule(b, a, 400, 1000) // B sees A as 500
	fmt.Println("A passes:", okA, "B passes:", okB, "total:", a+400+b+400)
}
