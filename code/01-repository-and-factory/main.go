// Factory: a whole aggregate, valid from its first instant.
// Repository: a collection of aggregates, one per root, saved in one transaction.
package main

import (
	"errors"
	"fmt"
)

// Wallet is the aggregate root.
type Wallet struct {
	ID       string
	Owner    string
	Balance  int
	Plan     string
	Payments []int
}

// Open is the factory. It checks every field before it builds anything.
func Open(id, owner, plan string) (*Wallet, error) {
	if owner == "" {
		return nil, errors.New("refused: owner missing")
	}
	return &Wallet{ID: id, Owner: owner, Plan: plan}, nil
}

// TopUp and Charge are the only doors to the balance.
func (w *Wallet) TopUp(n int) { w.Balance += n }
func (w *Wallet) Charge(n int) error {
	if n > w.Balance {
		return errors.New("refused: balance would be below 0")
	}
	w.Balance -= n
	w.Payments = append(w.Payments, n)
	return nil
}

// WalletRepository is the port: it looks like a collection.
type WalletRepository interface {
	Get(id string) *Wallet
	Save(w *Wallet)
}

// MemoryStore is one implementation behind the port, used in tests.
type MemoryStore struct{ rows map[string]Wallet }

func (s *MemoryStore) Get(id string) *Wallet { w := s.rows[id]; return &w }
func (s *MemoryStore) Save(w *Wallet)        { s.rows[w.ID] = *w } // whole aggregate, one write

// Tables is the failure: one repository per table, saved in pieces.
type Tables struct {
	WalletBalance map[string]int
	PaymentRows   map[string][]int
}

// SavePieces crashes between the two writes when crash is true.
func (t *Tables) SavePieces(w *Wallet, crash bool) {
	t.WalletBalance[w.ID] = w.Balance
	if crash {
		return
	}
	t.PaymentRows[w.ID] = w.Payments
}

// PaymentRepoSave goes around the root: nobody checks the rule.
func (t *Tables) PaymentRepoSave(id string, n int) {
	t.WalletBalance[id] -= n
}

func main() {
	_, err := Open("w-1", "", "trial")
	fmt.Println("empty owner:", err)
	w, _ := Open("w-1", "ana", "trial")
	fmt.Println("built:", w.Owner, w.Balance, w.Plan)
	var repo WalletRepository = &MemoryStore{rows: map[string]Wallet{}}
	repo.Save(w)
	got := repo.Get("w-1")
	got.TopUp(20)
	repo.Save(got)
	fmt.Println("after get, topUp, save:", repo.Get("w-1").Balance)
	t := &Tables{WalletBalance: map[string]int{"w-1": 20}, PaymentRows: map[string][]int{}}
	t.PaymentRepoSave("w-1", 30)
	fmt.Println("PaymentRepo.save 30 around the root:", t.WalletBalance["w-1"])
	c := repo.Get("w-1")
	fmt.Println("charge 15 through the root:", c.Charge(15), c.Balance)
	t2 := &Tables{WalletBalance: map[string]int{}, PaymentRows: map[string][]int{}}
	t2.SavePieces(c, true)
	fmt.Println("saved in pieces, crash: balance", t2.WalletBalance["w-1"], "payment rows", len(t2.PaymentRows["w-1"]))
}
