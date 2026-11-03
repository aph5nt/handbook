// Command shopify replays the pods lesson: a router by shop, a flash sale that
// stays inside one pod, a page that crosses pods, and a checker that fails the
// build on a call into another component's internals.
package main

import "fmt"

type Pod struct {
	Name     string
	From, To byte
	Capacity int
	Orders   map[string]int // buyer -> orders in this pod
}

func Route(pods []*Pod, shop string) *Pod {
	for _, p := range pods {
		if shop[0] >= p.From && shop[0] <= p.To {
			return p
		}
	}
	return nil
}

func Loads(pods []*Pod, reqs map[string]int) map[string]int {
	out := map[string]int{}
	for shop, n := range reqs {
		out[Route(pods, shop).Name] += n
	}
	return out
}

func Hot(load, capacity int) bool { return load*100 > capacity*80 }

func CrossShop(pods []*Pod, buyer string) int {
	total := 0
	for _, p := range pods {
		total += p.Orders[buyer]
	}
	return total
}

type Call struct{ FromComp, ToComp, Name string }

func Packwerk(private map[string]bool, calls []Call) error {
	for _, c := range calls {
		if c.FromComp != c.ToComp && private[c.ToComp+"."+c.Name] {
			return fmt.Errorf("privacy violation: %s calls %s.%s", c.FromComp, c.ToComp, c.Name)
		}
	}
	return nil
}

func main() {
	pods := []*Pod{
		{Name: "pod 1", From: 'A', To: 'F', Capacity: 1200, Orders: map[string]int{"buyer": 2}},
		{Name: "pod 2", From: 'G', To: 'M', Capacity: 1200, Orders: map[string]int{"buyer": 1}},
		{Name: "pod 3", From: 'N', To: 'Z', Capacity: 1200, Orders: map[string]int{"buyer": 4}},
	}
	fmt.Println("step 2: shop C goes to", Route(pods, "C").Name)
	reqs := map[string]int{}
	for s := byte('A'); s <= 'Z'; s++ {
		reqs[string(s)] = 10 // a model figure: 10 requests per second per shop
	}
	reqs["H"] *= 100
	loads := Loads(pods, reqs)
	for _, p := range pods {
		fmt.Printf("step 3-5: %s load %d of %d, hot: %v\n", p.Name, loads[p.Name], p.Capacity, Hot(loads[p.Name], p.Capacity))
	}
	fmt.Println("step 6: a buyer's orders across pods:", CrossShop(pods, "buyer"))
	private := map[string]bool{"Inventory.stockRow": true}
	fmt.Println("deploy step 3:", Packwerk(private, []Call{{"Checkout", "Inventory", "stockRow"}}))
	fmt.Println("deploy step 4:", Packwerk(private, []Call{{"Checkout", "Inventory", "Reserve"}}))
}
