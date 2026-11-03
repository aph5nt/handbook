// Eight fallacies: "latency is zero" and "the network is reliable" as code.
// A wallet page prices 300 tokens with one call each, then with one batched
// call. A payment call times out after it was applied, and a blind retry
// charges twice; an idempotency key makes the retry safe.
package main

import (
	"errors"
	"fmt"
)

// roundTripMs is the cost of one call across the network.
const roundTripMs = 5

// errTimeout means "I do not know": the call may or may not have been applied.
var errTimeout = errors.New("timeout")

// Network counts the time spent on round trips, and can lose one reply.
type Network struct {
	elapsedMs int
	dropReply bool // the service restarts after applying, the reply is lost
}

// Call sends one request. The handler always runs; the reply may be lost.
func (n *Network) Call(handler func() string) (string, error) {
	n.elapsedMs += roundTripMs
	reply := handler()
	if n.dropReply {
		n.dropReply = false
		return "", errTimeout // applied on the server, unknown to the client
	}
	return reply, nil
}

// PricesChatty makes one call per token: 300 tokens, 300 round trips.
func PricesChatty(n *Network, tokens int) {
	for t := 0; t < tokens; t++ {
		n.Call(func() string { return "price" })
	}
}

// PricesBatched asks for every price in one call.
func PricesBatched(n *Network, tokens int) {
	n.Call(func() string { return fmt.Sprintf("%d prices", tokens) })
}

// PaymentService applies charges. done remembers results by idempotency key.
type PaymentService struct {
	charges []string
	done    map[string]string
}

func (p *PaymentService) Charge(key, order string, amount int) string {
	if key != "" {
		if r, ok := p.done[key]; ok {
			return r // the same key again: return the stored result
		}
	}
	p.charges = append(p.charges, order)
	r := fmt.Sprintf("charged %d for %s", amount, order)
	if key != "" {
		p.done[key] = r
	}
	return r
}

// ChargeWithRetry retries once after a timeout. key "" is a blind retry.
func ChargeWithRetry(n *Network, p *PaymentService, key, order string) {
	_, err := n.Call(func() string { return p.Charge(key, order, 20) })
	if errors.Is(err, errTimeout) {
		n.Call(func() string { return p.Charge(key, order, 20) }) // retry
	}
}

func main() {
	n := &Network{}
	PricesChatty(n, 300)
	fmt.Printf("chatty: 300 calls x %d ms = %d ms\n", roundTripMs, n.elapsedMs)
	n = &Network{}
	PricesBatched(n, 300)
	fmt.Printf("batched: 1 call = %d ms\n", n.elapsedMs)

	p := &PaymentService{done: map[string]string{}}
	n = &Network{dropReply: true}
	ChargeWithRetry(n, p, "", "order-501")
	fmt.Printf("blind retry after a timeout: %d charges for 1 order\n", len(p.charges))

	p = &PaymentService{done: map[string]string{}}
	n = &Network{dropReply: true}
	ChargeWithRetry(n, p, "order-501", "order-501")
	fmt.Printf("retry with key order-501: %d charge for 1 order\n", len(p.charges))
}
