// Zero trust and least privilege. The same attacker shell in the web pod
// meets two clusters: a flat one with an admin token and no log, and a
// zero-trust one with a network allow-list, a scoped token, an audit log,
// grants that expire, and one ABAC rule.
package main

import (
	"fmt"
	"strings"
)

type token struct {
	subject string
	actions []string // e.g. "read"
	env     string   // "staging", or "*" for every environment
	until   int      // minute the grant ends; 0 means never
}

type cluster struct {
	netAllow map[string][]string // destination -> callers allowed to connect
	audit    []string            // nil means no audit log at all
}

// connect is the network check: on a flat network every caller gets in.
func (c *cluster) connect(from, to string) bool {
	if c.netAllow == nil {
		return true // flat network: no rule between pods
	}
	for _, f := range c.netAllow[to] {
		if f == from {
			return true
		}
	}
	return false
}

// call is the API check: the token must hold the action, the environment
// and an unexpired grant. Every answer is written to the audit log.
func (c *cluster) call(t token, action, env string, now int) int {
	code := 403
	if (t.until == 0 || now < t.until) && (t.env == "*" || t.env == env) && holds(t.actions, action) {
		code = 200
	}
	if c.audit != nil {
		c.audit = append(c.audit, fmt.Sprintf("%s %s %s -> %d", t.subject, action, env, code))
	}
	return code
}

func holds(actions []string, a string) bool {
	for _, x := range actions {
		if x == "*" || x == a {
			return true
		}
	}
	return false
}

// abacAllows is one attribute rule instead of a role per region per team.
func abacAllows(analystRegion, caseRegion string) bool { return analystRegion == caseRegion }

func main() {
	fmt.Println("Flat cluster, web pod token = admin, for now")
	flat := &cluster{}
	admin := token{subject: "web", actions: []string{"*"}, env: "*"}
	fmt.Println("  connect web -> db:5432   ->", flat.connect("web", "db"))
	fmt.Println("  delete production data   ->", flat.call(admin, "delete", "production", 10))
	fmt.Println("  audit lines              ->", len(flat.audit))

	fmt.Println("Zero-trust cluster")
	zt := &cluster{netAllow: map[string][]string{"db": {"api"}}, audit: []string{}}
	web := token{subject: "web", actions: []string{"read"}, env: "staging"}
	fmt.Println("  connect web -> db:5432   ->", zt.connect("web", "db"))
	fmt.Println("  write production         ->", zt.call(web, "write", "production", 10))
	fmt.Println("  read staging             ->", zt.call(web, "read", "staging", 11))
	fmt.Println("  audit log:\n    " + strings.Join(zt.audit, "\n    "))
	grant := token{subject: "ana", actions: []string{"*"}, env: "production", until: 60}
	fmt.Println("  migration admin, minute 30 ->", zt.call(grant, "alter", "production", 30))
	fmt.Println("  migration admin, minute 61 ->", zt.call(grant, "alter", "production", 61))
	fmt.Println("  ABAC eu analyst, eu case ->", abacAllows("eu", "eu"), " us case ->", abacAllows("eu", "us"))
}
