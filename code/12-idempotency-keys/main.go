// Idempotency keys: the story of the lesson 12-idempotency-keys.html.
// An agent tops up 10 USD. The charge succeeds but the reply is lost, so the
// agent retries. A key makes the retry return the stored receipt. No key, a
// key saved after the charge, or a key reused for a new body all go wrong.
package main

import (
	"crypto/sha256"
	"fmt"
)

type Entry struct {
	Done     bool
	BodyHash [32]byte
	Receipt  int
}

type API struct {
	Keys    map[string]*Entry
	Charges int
	next    int
}

// TopUp is one call. The key insert, the charge and the stored receipt are
// one transaction: either all of them happen or none.
func (a *API) TopUp(key, body string) (int, string) {
	h := sha256.Sum256([]byte(body))
	if key != "" {
		if e, ok := a.Keys[key]; ok { // the unique insert finds the key
			switch {
			case e.BodyHash != h:
				return 0, "422: key reused for a different body"
			case !e.Done:
				return 0, "409: in progress, try again shortly"
			default:
				return e.Receipt, "stored receipt, no new charge"
			}
		}
		a.Keys[key] = &Entry{BodyHash: h} // in progress
	}
	a.Charges++ // write the charge to the ledger
	a.next++
	receipt := 880 + a.next
	if key != "" {
		a.Keys[key].Done, a.Keys[key].Receipt = true, receipt
	}
	return receipt, "charged"
}

// TopUpWrongOrder charges first and saves the key in a separate step.
func (a *API) TopUpWrongOrder(key, body string, crash bool) {
	if _, ok := a.Keys[key]; ok {
		return
	}
	a.Charges++
	if crash {
		return // crashed between the charge and the key
	}
	a.Keys[key] = &Entry{Done: true, BodyHash: sha256.Sum256([]byte(body))}
}

func newAPI() *API { return &API{Keys: map[string]*Entry{}} }

func main() {
	a := newAPI()
	r, why := a.TopUp("K-7f3", "top up 10 USD")
	fmt.Printf("call 1: receipt %d, %s, reply lost\n", r, why)
	r, why = a.TopUp("K-7f3", "top up 10 USD")
	fmt.Printf("retry, same key: receipt %d, %s, charges = %d\n", r, why, a.Charges)
	a.Keys["K-8a1"] = &Entry{BodyHash: sha256.Sum256([]byte("top up 10 USD"))}
	_, why = a.TopUp("K-8a1", "top up 10 USD")
	fmt.Println("retry while the first is in progress:", why)

	fmt.Println("== No key, or a key done wrong")
	b := newAPI()
	b.TopUp("", "top up 10 USD")
	b.TopUp("", "top up 10 USD")
	fmt.Printf("no key, retried: charges = %d (paid 20 USD for 10)\n", b.Charges)
	c := newAPI()
	c.TopUpWrongOrder("K-7f3", "top up 10 USD", true)
	c.TopUpWrongOrder("K-7f3", "top up 10 USD", false)
	fmt.Printf("key saved after the charge, crash between: charges = %d\n", c.Charges)
	_, why = a.TopUp("K-7f3", "top up 50 USD")
	fmt.Printf("K-7f3 reused for 50 USD: %s, charges still %d\n", why, a.Charges)
}
