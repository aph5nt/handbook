// Egress control, as a decision function: nothing here opens a connection.
// Open cluster: everything leaves. Default deny: only DNS and the proxy,
// and the proxy holds a domain allow-list and raises an alert.
package main

import "fmt"

type namespace struct {
	defaultDeny bool
	allows      map[string]bool // destinations the NetworkPolicy lets out
}

type proxy struct {
	domains map[string]bool
	alerts  []string
}

// direct is the NetworkPolicy check for a connection leaving the pod.
func direct(n namespace, dest string) string {
	if !n.defaultDeny || n.allows[dest] {
		return "allowed"
	}
	return "dropped"
}

// through is the egress proxy: the name must be on the list. A refusal is
// logged and raises an alert, because a pod tried something unexpected.
func (p *proxy) through(pod, domain string) int {
	if p.domains[domain] {
		return 200
	}
	p.alerts = append(p.alerts, pod+" tried "+domain)
	return 403
}

// lookup needs its own allow: without it every name fails, not only bad ones.
func lookup(n namespace, name string) string {
	if direct(n, "cluster-dns") != "allowed" {
		return "lookup failed: " + name
	}
	return "address for " + name
}

func main() {
	fmt.Println("Open egress")
	open := namespace{}
	fmt.Println("  bad code, lookup   ->", lookup(open, "attacker.example"))
	fmt.Println("  bad code, POST env ->", direct(open, "attacker.example"), "(no refusal, no alert)")
	fmt.Println("  real price fetch   ->", direct(open, "api.prices.example"))

	fmt.Println("Default deny")
	n := namespace{true, map[string]bool{"cluster-dns": true, "egress-proxy": true}}
	p := &proxy{domains: map[string]bool{"api.prices.example": true}}
	fmt.Println("  bad code, direct   ->", direct(n, "attacker.example"))
	fmt.Println("  bad code, proxy    ->", p.through("pricing", "attacker.example"))
	fmt.Println("  real fetch, proxy  ->", p.through("pricing", "api.prices.example"))
	fmt.Println("  alert              ->", p.alerts)
	broken := namespace{true, map[string]bool{"egress-proxy": true}}
	fmt.Println("  no DNS allow       ->", lookup(broken, "api.prices.example"))
}
