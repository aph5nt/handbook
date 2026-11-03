// Memoization: a cache inside one function.
// Replays the lesson: decimals(token) is pure and safe to memoize,
// balance(address) is not, until the block number joins the key.
package main

import (
	"container/list"
	"fmt"
)

// Memo maps a key (the arguments) to a result, bounded by LRU eviction.
type Memo[K comparable, V any] struct {
	limit int
	order *list.List // front = most recently used
	items map[K]*list.Element
}

type entry[K comparable, V any] struct {
	key K
	val V
}

func NewMemo[K comparable, V any](limit int) *Memo[K, V] {
	return &Memo[K, V]{limit: limit, order: list.New(), items: map[K]*list.Element{}}
}

// Get returns the saved result, or runs load once and saves it.
func (m *Memo[K, V]) Get(key K, load func(K) V) (V, bool) {
	if el, ok := m.items[key]; ok { // hit: the arguments were seen before
		m.order.MoveToFront(el)
		return el.Value.(entry[K, V]).val, true
	}
	v := load(key) // miss: do the real work once
	m.items[key] = m.order.PushFront(entry[K, V]{key, v})
	if m.order.Len() > m.limit { // bound the map: drop the least recently used
		old := m.order.Remove(m.order.Back()).(entry[K, V])
		delete(m.items, old.key)
	}
	return v, false
}

func (m *Memo[K, V]) Len() int { return m.order.Len() }

// The chain node: every call is a slow network round trip.
type node struct {
	calls    int
	decimals map[string]int
	balance  map[string]int // ETH at the current head
}

func (n *node) Decimals(token string) int { n.calls++; return n.decimals[token] }
func (n *node) Balance(addr string) int   { n.calls++; return n.balance[addr] }

type balanceKey struct {
	addr  string
	block int
}

func main() {
	n := &node{decimals: map[string]int{"USDC": 6, "WETH": 18}, balance: map[string]int{"0x18": 5}}

	fmt.Println("== pure function: decimals(token) ==")
	dec := NewMemo[string, int](10_000)
	v, hit := dec.Get("USDC", n.Decimals)
	fmt.Printf("decimals(USDC) = %d  hit=%v  node calls=%d  entries=%d\n", v, hit, n.calls, dec.Len())
	for i := 0; i < 1000; i++ {
		dec.Get("USDC", n.Decimals)
	}
	fmt.Printf("after 1,000 more USDC transfers: node calls=%d\n", n.calls)
	v, hit = dec.Get("WETH", n.Decimals)
	fmt.Printf("decimals(WETH) = %d  hit=%v  node calls=%d  entries=%d\n", v, hit, n.calls, dec.Len())
	fmt.Printf("1,002 transfers decoded, node asked %d times, limit %d entries (LRU)\n\n", n.calls, dec.limit)

	fmt.Println("== not pure: balance(address) ==")
	n.calls = 0
	bal := NewMemo[string, int](10_000)
	v, _ = bal.Get("0x18", n.Balance)
	fmt.Printf("balance(0x18) = %d ETH  node calls=%d\n", v, n.calls)
	n.balance["0x18"] = 2 // a new block sends 3 ETH out
	v, hit = bal.Get("0x18", n.Balance)
	fmt.Printf("after the block: memo says %d ETH (hit=%v), node says %d ETH  -> WRONG forever\n", v, hit, n.balance["0x18"])

	fmt.Println("\n== fix: the block number joins the arguments ==")
	atBlock := map[balanceKey]int{{"0x18", 9506}: 5, {"0x18", 9507}: 2}
	byBlock := NewMemo[balanceKey, int](10_000)
	load := func(k balanceKey) int { n.calls++; return atBlock[k] }
	v, _ = byBlock.Get(balanceKey{"0x18", 9506}, load)
	fmt.Printf("balance(0x18, block 9506) = %d ETH\n", v)
	v, hit = byBlock.Get(balanceKey{"0x18", 9507}, load)
	fmt.Printf("balance(0x18, block 9507) = %d ETH  hit=%v  -> a new key, so a fresh answer\n", v, hit)
}
