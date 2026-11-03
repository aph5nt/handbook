// Strategic design: decide which context owns an idea before writing a class.
// Tactical design: shape the code inside that one context.
package main

import "fmt"

// Strategic: the context map. Each idea has exactly one owner.
type ContextMap struct {
	Owner map[string]string   // idea -> owning context
	Edges map[string][]string // context -> contexts it supplies
}

// Claim is how a context starts keeping an idea. A second owner is a defect.
func (m *ContextMap) Claim(idea, ctx string) error {
	if o, ok := m.Owner[idea]; ok && o != ctx {
		return fmt.Errorf("%s already owned by %s", idea, o)
	}
	m.Owner[idea] = ctx
	return nil
}

// Tactical, inside pricing only.

// DayPrice is a value object: equal when its values are equal.
type DayPrice struct {
	Token, Day string
	USD        float64
}

// Token is an entity: same ID even when its name or decimals change.
type Token struct {
	ID       string
	Name     string
	Decimals int
}

// PriceImported is a domain event other contexts may learn about.
type PriceImported struct{ Price DayPrice }

// USD is the one rule for a transfer's value, owned by pricing.
func USD(amount float64, p DayPrice) float64 { return amount * p.USD }

func main() {
	m := &ContextMap{Owner: map[string]string{}, Edges: map[string][]string{}}
	fmt.Println("pricing claims prices:", m.Claim("price", "pricing"))
	m.Edges["vendor"] = []string{"pricing"} // through a translator
	m.Edges["pricing"] = []string{"graph", "billing"}
	m.Edges["indexer"] = []string{"graph"}
	fmt.Println("graph claims prices too:", m.Claim("price", "graph"))
	p := DayPrice{Token: "ETH", Day: "2026-10-01", USD: 2000}
	ev := PriceImported{Price: p}
	fmt.Println("event:", ev.Price.Token, ev.Price.Day)
	t := Token{ID: "eth", Name: "Ether", Decimals: 18}
	t.Name = "ETH"
	fmt.Println("entity keeps its id:", t.ID)
	fmt.Println("graph and billing read pricing:", USD(2.5, p), USD(2.5, p))
	// tactical first: three Price classes, three rules, three answers
	rules := map[string]float64{"indexer": 2000, "graph": 1980, "billing": 2015}
	for _, ctx := range []string{"indexer", "graph", "billing"} {
		fmt.Println("tactical first,", ctx, "says", 2.5*rules[ctx])
	}
}
