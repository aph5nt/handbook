// Threat modelling and audit logs. Tab 1: walk STRIDE over each trust
// boundary, find the open fetch, fix it, and refuse a boundary nobody
// walked. Tab 2: an append-only audit log of identifiers, never values.
package main

import (
	"crypto/sha256"
	"fmt"
	"net/netip"
	"strings"
)

type boundary struct {
	from, to   string
	createdDay int
	walkedDay  int // 0 means never walked
}

type finding struct{ letter, question, owner, reviewDay string }

// walk asks the six STRIDE questions for one boundary. Every answer that is
// not yet a control becomes a finding with an owner or a review date.
func walk(b boundary, controls map[string]string) []finding {
	questions := map[string]string{"S": "who is calling?", "T": "what signs the data?",
		"R": "what is logged?", "I": "what leaves, and to whom?",
		"D": "what limits exist?", "E": "what can a lower identity reach?"}
	var out []finding
	for _, l := range []string{"S", "T", "R", "I", "D", "E"} {
		if controls[l] == "" {
			out = append(out, finding{l, questions[l], "owner: proxy team", ""})
		}
	}
	return out
}

// fetchAllowed is the I fix: the read-only fetch tool refuses private ranges.
func fetchAllowed(ip string) bool {
	a := netip.MustParseAddr(ip)
	return !(a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast())
}

// unwalked is the failure check: a boundary created after its last walk.
func unwalked(bs []boundary) []string {
	var out []string
	for _, b := range bs {
		if b.walkedDay < b.createdDay {
			out = append(out, b.from+" -> "+b.to)
		}
	}
	return out
}

// ---------- Tab 2: audit log ----------

type auditLog struct{ lines []string } // append only: there is no delete

// record writes caller, action, target, result. Secrets are reduced to an id.
func (l *auditLog) record(caller, action, target, result, token string) {
	id := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))[:4]
	l.lines = append(l.lines, fmt.Sprintf("%s token:%s %s %s %s", caller, id, action, target, result))
}

// recordCareless is the failure: the full token and a person's name.
func (l *auditLog) recordCareless(caller, action, token, ownerName string) {
	l.lines = append(l.lines, fmt.Sprintf("%s %s token=%s owner=%s", caller, action, token, ownerName))
}

func (l *auditLog) find(word string) []string {
	var out []string
	for _, s := range l.lines {
		if strings.Contains(s, word) {
			out = append(out, s)
		}
	}
	return out
}

func main() {
	fmt.Println("Tab 1: STRIDE over host -> proxy")
	b := boundary{"agent host", "tool proxy", 1, 1}
	controls := map[string]string{"S": "token per host", "T": "signed releases", "R": "audit log"}
	for _, f := range walk(b, controls) {
		fmt.Printf("  %s %-35s %s\n", f.letter, f.question, f.owner)
	}
	fmt.Println("  I: before the fix, fetch http://10.0.0.5/admin -> internal page (no check)")
	fmt.Println("  I: after the fix, allowed?", fetchAllowed("10.0.0.5"), " public 93.184.215.14?", fetchAllowed("93.184.215.14"))
	controls["I"], controls["D"], controls["E"] = "private ranges blocked", "rate limit per host", "production read only"
	fmt.Println("  open findings after the fixes:", len(walk(b, controls)))
	bs := []boundary{b, {"proxy", "payments API", 30, 0}}
	fmt.Println("  day 30, boundaries never walked:", unwalked(bs))

	fmt.Println("Tab 2: audit log")
	log := &auditLog{}
	log.record("agent-7", "write", "staging/config", "ok", "eyJhbGciOiJSUzI1NiJ9.secret")
	log.record("agent-7", "write", "production/config", "denied", "eyJhbGciOiJSUzI1NiJ9.secret")
	fmt.Println("  who changed staging?", log.find("staging"))
	fmt.Println("  denied calls:", log.find("denied"))
	careless := &auditLog{}
	careless.recordCareless("agent-7", "read case 202", "eyJhbGciOiJSUzI1NiJ9.secret", "Jan Kowalski")
	fmt.Println("  careless line:", careless.lines[0])
	fmt.Println("  fixed line:   ", log.lines[0])
}
