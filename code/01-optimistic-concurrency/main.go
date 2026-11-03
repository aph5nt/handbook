// Optimistic concurrency: no lock is held while a request thinks. The save
// only matches if nobody else saved since this request loaded.
package main

import (
	"errors"
	"fmt"
)

// Row is the stored wallet.
type Row struct{ Balance, Version int }

// Copy is what one request holds in memory.
type Copy struct{ Balance, Version int }

var errConflict = errors.New("conflict: zero rows matched")

// Load remembers the version. No lock.
func Load(r *Row) Copy { return Copy{r.Balance, r.Version} }

// Charge runs the invariant on the request's own copy.
func (c *Copy) Charge(n int) error {
	if n > c.Balance {
		return errors.New("refused: not enough")
	}
	c.Balance -= n
	return nil
}

// Save is: UPDATE wallet SET balance=?, version=version+1 WHERE version=?
func Save(r *Row, c Copy) error {
	if r.Version != c.Version {
		return errConflict // nothing overwritten
	}
	r.Balance = c.Balance
	r.Version++
	return nil
}

// SaveBlind is the failure: no version check, the last write wins.
func SaveBlind(r *Row, c Copy) { r.Balance = c.Balance }

// ChargeWithRetry reloads and runs the command again on the fresh state.
func ChargeWithRetry(r *Row, c Copy, n int) error {
	if err := Save(r, c); !errors.Is(err, errConflict) {
		return err
	}
	fresh := Load(r) // never re-send the old result
	if err := fresh.Charge(n); err != nil {
		return err
	}
	return Save(r, fresh)
}

func main() {
	r := &Row{Balance: 20, Version: 7}
	a, b := Load(r), Load(r)
	fmt.Println("A and B hold:", a, b)
	a.Charge(15)
	fmt.Println("A saves:", Save(r, a), "row", *r)
	b.Charge(10)
	fmt.Println("B saves:", Save(r, b), "row", *r)
	fmt.Println("B retries:", ChargeWithRetry(r, b, 10), "row", *r)
	r2 := &Row{Balance: 20}
	a2, b2 := Load(r2), Load(r2)
	a2.Charge(15)
	b2.Charge(10)
	SaveBlind(r2, a2)
	SaveBlind(r2, b2)
	fmt.Println("no version: charged 25 in total, wallet says", r2.Balance)
}
