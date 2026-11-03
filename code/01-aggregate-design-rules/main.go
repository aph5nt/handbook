// Aggregate design: the border holds only what the invariant reads. Everything
// else is held by id, kept outside, and changed in its own transaction.
package main

import "fmt"

// BigWallet is the first draft: everything the wallet touches, one version.
type BigWallet struct {
	Version     int
	Balance     int
	ProfileName string
	Plan        string
	Payments    []int
}

// LoadCost counts the items a charge must load.
func (w BigWallet) LoadCost() int { return 3 + len(w.Payments) }

// Wallet is the small aggregate: rule 1, only what the invariant reads.
// Rule 2: other aggregates are held by id.
type Wallet struct {
	Version        int
	Balance        int
	CustomerID     string
	SubscriptionID string
}

// LoadCost: the balance and two ids. No payments.
func (w Wallet) LoadCost() int { return 3 }

// Customer is its own aggregate with its own version.
type Customer struct {
	Version int
	Name    string
}

// Ledger is rule 3: an unbounded collection lives outside. Append only.
type Ledger struct{ Rows []int }

func (l *Ledger) Add(n int) { l.Rows = append(l.Rows, n) }

// Charge is rule 4: one transaction changes one aggregate; others hear an event.
func Charge(w *Wallet, l *Ledger, n int) []string {
	w.Balance -= n
	l.Add(n)
	return []string{"WalletCharged"}
}

// save stores an edit only if nobody saved since this edit loaded.
func save(stored *int, loaded int) bool {
	if *stored != loaded {
		return false // a version conflict
	}
	*stored++
	return true
}

func main() {
	pay := make([]int, 10000)
	big := BigWallet{Version: 7, Balance: 20, Payments: pay}
	fmt.Println("first draft loads per charge:", big.LoadCost(), "items")
	nameOK := save(&big.Version, 7)
	chargeOK := save(&big.Version, 7) // loaded before the name edit saved
	fmt.Println("one version: name edit saved", nameOK, "- charge saved", chargeOK)
	w := &Wallet{Version: 7, Balance: 20, CustomerID: "c-17", SubscriptionID: "s-3"}
	c := &Customer{Version: 4, Name: "Anna"}
	l := &Ledger{Rows: pay}
	fmt.Println("small wallet loads per charge:", w.LoadCost(), "items")
	c.Name = "Anna N"
	nameOK = save(&c.Version, 4)
	ev := Charge(w, l, 5)
	chargeOK = save(&w.Version, 7)
	fmt.Println("two versions: name edit saved", nameOK, "- charge saved", chargeOK)
	fmt.Println("ledger rows:", len(l.Rows), "event:", ev[0])
}
