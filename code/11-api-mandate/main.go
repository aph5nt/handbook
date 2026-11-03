// Command mandate replays the API mandate lesson: a column rename that breaks
// another team through a shared table, then the same rename hidden behind an
// API, a remote call with no timeout, and the timeout with a fallback.
package main

import "fmt"

type Row map[string]int

// Before: the cart reads a table that another team owns.
func CartReadShared(items map[int]Row, item int) (int, error) {
	v, ok := items[item]["price"]
	if !ok {
		return 0, fmt.Errorf("column price does not exist")
	}
	return v, nil
}

func RenameColumn(items map[int]Row, from, to string) {
	for _, r := range items {
		r[to] = r[from]
		delete(r, from)
	}
}

// After: only the pricing service touches its table.
type Pricing struct {
	table     map[int]Row
	LatencyMs int
	Pages     int
}

func (p *Pricing) GetPrice(item int) (price, latencyMs int) {
	if p.LatencyMs > 1000 {
		p.Pages++
	}
	return p.table[item]["list_price"], p.LatencyMs
}

type Cart struct {
	TimeoutMs int // 0 means no timeout
	LastPrice map[int]int
}

func (c *Cart) Checkout(p *Pricing, item int) (price, waitedMs int, note string) {
	v, ms := p.GetPrice(item)
	if c.TimeoutMs == 0 {
		return v, ms, "waited for pricing"
	}
	if ms <= c.TimeoutMs {
		c.LastPrice[item] = v
		return v, ms, "fresh"
	}
	return c.LastPrice[item], c.TimeoutMs, "may be old"
}

func main() {
	fmt.Println("before tab")
	items := map[int]Row{1: {"price": 99}}
	v, err := CartReadShared(items, 1)
	fmt.Println("step 1: cart reads price:", v, err)
	RenameColumn(items, "price", "list_price")
	fmt.Println("step 2: pricing renames price to list_price; pricing's table:", items[1])
	_, err = CartReadShared(items, 1)
	fmt.Println("step 3: cart in December:", err)

	fmt.Println("after tab")
	p := &Pricing{table: items, LatencyMs: 50}
	c := &Cart{TimeoutMs: 0, LastPrice: map[int]int{}}
	v, ms, n := c.Checkout(p, 1)
	fmt.Printf("step 2: through the API: price %d in %d ms (%s)\n", v, ms, n)
	p.LatencyMs = 5000
	v, ms, n = c.Checkout(p, 1)
	fmt.Printf("step 3-4: pricing takes 5000 ms, no timeout: price %d, checkout waited %d ms (%s), pricing pages %d\n", v, ms, n, p.Pages)
	c.TimeoutMs = 200
	c.LastPrice[1] = 99
	v, ms, n = c.Checkout(p, 1)
	fmt.Printf("step 5: timeout 200 ms: price %d, checkout waited %d ms (%s)\n", v, ms, n)
}
