// Bounded context: the border goes where a word changes meaning. Each context
// has its own small model. Contexts share ids and events, never objects.
package main

import "fmt"

// Shared is the god model: one Customer table, read by three teams.
type Shared struct {
	Columns map[string][]string // column -> teams that read it
}

// Rename in a shared model breaks every other reader of that column.
func (s Shared) Rename(team, col string) []string {
	var broken []string
	for _, t := range s.Columns[col] {
		if t != team {
			broken = append(broken, t)
		}
	}
	return broken
}

// Three small models: same person, one meaning each.
type Prospect struct {
	ID, Name  string
	Preferred []string
}
type Payer struct {
	ID, BillingAddress, Terms string
}
type Consignee struct {
	ID, DeliveryAddress, Window string
}

// OrderPlaced crosses the border: an id, never an object.
type OrderPlaced struct{ CustomerID string }

// Billing handles the event in its own words.
func Billing(e OrderPlaced, payers map[string]Payer) Payer { return payers[e.CustomerID] }

// KundeRow is the old ERP's table, in its own language.
type KundeRow struct {
	KDNr     string
	Zahlziel int
}

// Translate is the anti-corruption layer: legacy names stop here.
func Translate(k KundeRow) Payer {
	return Payer{ID: k.KDNr, Terms: fmt.Sprintf("%d days", k.Zahlziel)}
}

func main() {
	s := Shared{Columns: map[string][]string{"address": {"sales", "billing", "shipping"}}}
	fmt.Println("billing renames address, broken:", s.Rename("billing", "address"))
	payers := map[string]Payer{"c-17": {ID: "c-17", BillingAddress: "Warsaw", Terms: "30 days"}}
	payers["c-17"] = Payer{ID: "c-17", BillingAddress: "Warsaw HQ", Terms: "30 days"} // billing's own field
	own := Shared{Columns: map[string][]string{"billing_address": {"billing"}}}       // Payer's only reader
	fmt.Println("billing renames its own Payer field, broken:", len(own.Rename("billing", "billing_address")))
	fmt.Println("OrderPlaced c-17 in billing:", Billing(OrderPlaced{"c-17"}, payers).BillingAddress)
	fmt.Println("Prospect and Consignee are other models:", Prospect{ID: "c-17"}.ID, Consignee{ID: "c-17"}.ID)
	fmt.Printf("KUNDE row translated: %+v\n", Translate(KundeRow{"c-17", 30}))
}
