// A domain event is private to its context. A translator turns it into an
// integration event: a small, versioned public contract.
package main

import "fmt"

// WalletToppedUp is the domain event. Billing may change it in any commit.
// Kept as a map so a rename can really happen in this program.
type WalletToppedUp map[string]any

// PaymentReceivedV1 is the integration event: few fields, stable names.
type PaymentReceivedV1 struct {
	Version    int
	CustomerID string
	USD        int
	ReceivedAt string
}

// amountField is what Billing calls the amount today. A rename changes it here.
var amountField = "amount"

// Renew is the handler inside the context, changed in the same commit.
func Renew(e WalletToppedUp) string {
	return fmt.Sprintf("subscription renewed for %v USD", e[amountField])
}

// Translate maps the private event to the public contract.
func Translate(e WalletToppedUp) PaymentReceivedV1 {
	return PaymentReceivedV1{1, e["customerId"].(string), e[amountField].(int), e["at"].(string)}
}

// Email and Analytics read only the contract.
func Email(p PaymentReceivedV1) string     { return fmt.Sprintf("receipt: %d USD", p.USD) }
func Analytics(p PaymentReceivedV1) string { return fmt.Sprintf("revenue: %d", p.USD) }

// EmailRaw is the failure: another team reads the private event by its field name.
func EmailRaw(e WalletToppedUp) string {
	usd, _ := e["amount"].(int) // missing after the rename: zero
	return fmt.Sprintf("receipt: %d USD", usd)
}

func event() WalletToppedUp {
	return WalletToppedUp{"walletId": "w-7", "customerId": "c-17", amountField: 20, "balanceAfter": 20, "at": "2026-10-01"}
}

func main() {
	e := event()
	fmt.Println(Renew(e))
	p := Translate(e)
	fmt.Printf("contract: %+v\n", p)
	fmt.Println(Email(p), "-", Analytics(p))
	amountField = "money" // Billing renames, handler and translator in one commit
	e = event()
	fmt.Println(Renew(e))
	fmt.Printf("contract after the rename: %+v\n", Translate(e))
	fmt.Println("raw event after the rename:", EmailRaw(e))
}
