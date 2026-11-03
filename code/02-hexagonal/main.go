// Replays the hexagonal lesson: one order core, two ports, adapters that plug
// in from outside, and the leak of the discount rule into the REST adapter.
// The Postgres adapter prints its statement, so the program needs no database.
package main

import "fmt"

const phonePrice = 200 // EUR

// Order is the core's own word for an order.
type Order struct {
	Qty   int
	Total int // EUR
}

// PlaceOrder is the port for requests coming in. The core owns it.
type PlaceOrder interface {
	Place(qty int) Order
}

// OrderStore is the port for saving. The core owns it too.
type OrderStore interface {
	Save(o Order)
}

// Core holds the rule and imports nothing from outside.
type Core struct{ store OrderStore }

// Place applies the rule, then calls a port the core owns.
func (c Core) Place(qty int) Order {
	subtotal := qty * phonePrice // 3 x 200 = 600 EUR
	total := subtotal
	if subtotal > 500 { // the rule: over 500 EUR, 10 percent off
		total = subtotal * 90 / 100
	}
	o := Order{Qty: qty, Total: total}
	c.store.Save(o)
	return o
}

// restAdapter translates POST /orders into a port call. It holds no rule.
func restAdapter(p PlaceOrder, bodyQty int) Order { return p.Place(bodyQty) }

// cliAdapter translates "shop order" with a quantity into the same port call.
func cliAdapter(p PlaceOrder, flagQty int) Order { return p.Place(flagQty) }

// postgresAdapter implements OrderStore.
type postgresAdapter struct{}

func (postgresAdapter) Save(o Order) {
	fmt.Printf("  postgres: INSERT INTO orders (qty, total) VALUES (%d, %d)\n", o.Qty, o.Total)
}

// memoryAdapter implements OrderStore with a map, for tests.
type memoryAdapter struct{ orders map[int]Order }

func (m memoryAdapter) Save(o Order) { m.orders[len(m.orders)+1] = o }

// leakyCore is the failure tab: the rule was moved out of the core.
type leakyCore struct{ store OrderStore }

func (c leakyCore) Place(qty int) Order {
	o := Order{Qty: qty, Total: qty * phonePrice} // no rule line here any more
	c.store.Save(o)
	return o
}

// leakyREST holds the rule, so only the web gets the discount.
func leakyREST(store OrderStore, bodyQty int) Order {
	total := bodyQty * phonePrice
	if total > 500 { // the rule, in the wrong place
		total = total * 90 / 100
	}
	o := Order{Qty: bodyQty, Total: total}
	store.Save(o)
	return o
}

func main() {
	fmt.Println("tab 1: the rule in the core")
	web := Core{store: postgresAdapter{}}
	fmt.Printf("  web, 3 phones: subtotal 600, total %d EUR\n", restAdapter(web, 3).Total)
	fmt.Printf("  command line, 3 phones: total %d EUR\n", cliAdapter(web, 3).Total)

	mem := memoryAdapter{orders: map[int]Order{}}
	swapped := Core{store: mem}
	fmt.Printf("  swap to the map store: total %d EUR, orders in the map %d\n", restAdapter(swapped, 3).Total, len(mem.orders))

	got := swapped.Place(3)
	fmt.Printf("  test with no network: want 540, got %d, pass %v\n", got.Total, got.Total == 540)

	fmt.Println("tab 2: the rule leaks into the REST adapter")
	leakMem := memoryAdapter{orders: map[int]Order{}}
	leaky := leakyCore{store: leakMem}
	webTotal := leakyREST(leakMem, 3).Total
	cliTotal := cliAdapter(leaky, 3).Total
	fmt.Printf("  web: %d EUR, command line: %d EUR, overcharged %d EUR\n", webTotal, cliTotal, cliTotal-webTotal)
	testGot := leaky.Place(3)
	fmt.Printf("  test: want 540, got %d, pass %v\n", testGot.Total, testGot.Total == 540)

	fixed := Core{store: leakMem}
	fmt.Printf("  fix, rule back in the core: web %d, command line %d, test %d EUR\n",
		restAdapter(fixed, 3).Total, cliAdapter(fixed, 3).Total, fixed.Place(3).Total)
}
