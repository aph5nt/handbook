// Replays the gateway, BFF and service mesh lesson. North-south: a gateway
// that checks the key, counts calls and holds the free-tier limit, and two
// BFFs that reshape answers. East-west: direct calls under a network policy,
// then sidecars that add mutual TLS, a trace id, one retry and a timeout.
// Time is simulated in milliseconds, so every run prints the same numbers.
package main

import (
	"errors"
	"fmt"
)

// ---------- north-south: the gateway

type Gateway struct {
	keys    map[string]bool // known caller keys
	limit   int             // free-tier calls per caller
	counted map[string]int  // calls counted per caller
	refused int
}

// Handle is the front door: check, count, limit, route. Nothing else.
func (g *Gateway) Handle(key string, call func() string) (string, error) {
	if !g.keys[key] {
		return "", errors.New("unknown key")
	}
	if g.counted[key] >= g.limit { // free tier used up
		g.refused++
		return "", errors.New("refused: free tier used up")
	}
	g.counted[key]++ // metering
	return call(), nil
}

type Risk struct{}

func (r *Risk) Score(addr string) string { return addr + ": risk 0.12" }

type Wallet struct{}

func (Wallet) Balance(addr string) string { return addr + ": 3 tokens" }

// webBFF joins two answers into the dashboard shape. It decides nothing.
func webBFF(g *Gateway, addr string) []string {
	r, _ := g.Handle("web", func() string { return risk.Score(addr) })
	w, _ := g.Handle("web", func() string { return Wallet{}.Balance(addr) })
	return []string{r, w, "chart of the last 30 days", "full history table"}
}

// mobileBFF calls the same services and returns a smaller shape.
func mobileBFF(g *Gateway, addr string) []string {
	r, _ := g.Handle("mobile", func() string { return risk.Score(addr) })
	return []string{r}
}

var risk = &Risk{}

// placesHoldingRule is the check behind the failure step: where does the
// business rule live?
func placesHoldingRule(rule string, holders map[string][]string) []string {
	var out []string
	for _, name := range []string{"gateway", "web BFF", "risk service"} {
		for _, r := range holders[name] {
			if r == rule {
				out = append(out, name)
			}
		}
	}
	return out
}

func northSouth() {
	fmt.Println("tab 1: gateway and BFFs")
	g := &Gateway{keys: map[string]bool{"agent": true, "web": true, "mobile": true},
		limit: 3, counted: map[string]int{}}
	for i := 1; i <= 4; i++ {
		_, err := g.Handle("agent", func() string { return risk.Score("0xabc") })
		if err != nil {
			fmt.Printf("  agent call %d: %v (counted %d of %d)\n", i, err, g.counted["agent"], g.limit)
		} else {
			fmt.Printf("  agent call %d: passed, counted %d of %d\n", i, g.counted["agent"], g.limit)
		}
	}
	fmt.Printf("  refused at the door: %d, risk service saw %d calls\n", g.refused, g.counted["agent"])
	fmt.Printf("  web BFF shape: %d fields, mobile BFF shape: %d field\n",
		len(webBFF(g, "0xabc")), len(mobileBFF(g, "0xabc")))

	rule := "block risky wallets"
	smart := map[string][]string{"gateway": {rule}, "web BFF": {rule}, "risk service": {rule}}
	fmt.Printf("  smart gateway: rule held by %v = %d places\n", placesHoldingRule(rule, smart), len(placesHoldingRule(rule, smart)))
	dumb := map[string][]string{"risk service": {rule}}
	fmt.Printf("  dumb gateway: rule held by %v = %d place\n", placesHoldingRule(rule, dumb), len(placesHoldingRule(rule, dumb)))
}

// ---------- east-west: direct calls, then the mesh

var services = []string{"risk", "wallet", "pricing", "graph", "alerts"}
var edges = [][2]string{{"risk", "wallet"}, {"risk", "pricing"}, {"alerts", "risk"}, {"graph", "pricing"}}

// allowed is the Kubernetes network policy: who may call whom.
var allowed = map[[2]string]bool{}

// Sidecar is the proxy next to each service. The control plane sets every field.
type Sidecar struct {
	cert      int // certificate version this sidecar presents
	trusts    int // certificate version this sidecar accepts
	timeoutMs int
	retries   int
}

// call sends one request through two sidecars and returns the time spent.
func (from *Sidecar) call(to *Sidecar, latencyMs int, trace string) (int, error) {
	if to.trusts != from.cert || from.trusts != to.cert { // mutual TLS: both sides check
		return 0, errors.New("mutual TLS failed")
	}
	spent := 0
	for attempt := 0; attempt <= from.retries; attempt++ {
		if latencyMs <= from.timeoutMs {
			return spent + latencyMs, nil
		}
		spent += from.timeoutMs // wait, give up on this attempt
		fmt.Printf("    %s attempt %d timed out after %d ms\n", trace, attempt+1, from.timeoutMs)
	}
	return spent, errors.New("timeout")
}

func eastWest() {
	fmt.Println("tab 2: service mesh")
	for _, e := range edges {
		allowed[e] = true
	}
	ok := 0
	for _, e := range edges {
		if allowed[e] {
			ok++
		}
	}
	fmt.Printf("  direct calls allowed by network policy: %d of %d, proxy hops per call: 0\n", ok, len(edges))
	fmt.Printf("  blocked by policy: wallet -> graph allowed? %v\n", allowed[[2]string{"wallet", "graph"}])

	sc := map[string]*Sidecar{}
	for _, s := range services {
		sc[s] = &Sidecar{cert: 1, trusts: 1, timeoutMs: 200, retries: 1}
	}
	ms, err := sc["risk"].call(sc["wallet"], 40, "trace-1")
	fmt.Printf("  risk -> wallet through 2 sidecars (proxy hops per call: 2): %d ms, err %v\n", ms, err)
	ms, err = sc["risk"].call(sc["wallet"], 500, "trace-2") // wallet slows to 500 ms
	fmt.Printf("  wallet slow (500 ms): gave up after %d ms, err %v\n", ms, err)

	// the control plane upgrade: every sidecar now trusts only version 2,
	// but the new certificates were never issued, so each still presents 1
	for _, s := range services {
		sc[s].trusts = 2
	}
	failing := 0
	for _, e := range edges {
		if _, err := sc[e[0]].call(sc[e[1]], 40, "trace-x"); err != nil {
			failing++
		}
	}
	fmt.Printf("  after the bad control plane push: %d of %d calls between services fail\n", failing, len(edges))
}

func main() {
	northSouth()
	eastWest()
}
