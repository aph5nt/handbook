// The server asks back: sampling, roots and elicitation.
// Each one exists only if the client declared it in initialize. Run: go run .
package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

type Caps struct{ Sampling, Roots, Elicitation bool }

var errNoMethod = errors.New("error -32601 method not found")

// Host is the client side. It answers requests the server sends back.
type Host struct {
	caps    Caps
	approve bool
}

func (h *Host) CreateMessage(prompt string) (string, error) {
	if !h.caps.Sampling {
		return "", errNoMethod
	}
	if !h.approve { // the user sees the request first
		return "", errors.New("user refused")
	}
	return "summary: 3 senders hold 90% of the volume", nil
}

// Sampling: summarise 5,000 rows with the host's model, or page if absent.
func summarise(h *Host, rows int, checkFirst bool) string {
	if checkFirst && !h.caps.Sampling {
		return fmt.Sprintf("50 rows + total %d + cursor", rows)
	}
	out, err := h.CreateMessage(fmt.Sprintf("summarise %d rows", rows))
	if err != nil {
		return "tool failed: " + err.Error()
	}
	return out
}

// Roots: a cooperating server checks each path against the roots.
func insideRoots(roots []string, path string) bool {
	for _, r := range roots {
		rel, err := filepath.Rel(r, path)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return true
		}
	}
	return false
}

// A sandbox is enforced by the operating system, not by the server.
func sandboxOpen(visible []string, path string) string {
	if !insideRoots(visible, path) {
		return "no such file (sandbox)"
	}
	return "contents"
}

// Elicitation: a flat form. Only accept carries content.
type Answer struct {
	Action string // accept, decline, cancel
	Amount int
}

func topup(form []string, ans Answer) string {
	for _, f := range form {
		if strings.Contains(f, "key") || strings.Contains(f, "password") {
			return "forbidden: field " + f + " asks for a secret"
		}
	}
	if ans.Action != "accept" {
		return ans.Action + ": nothing spent"
	}
	return fmt.Sprintf("topped up %d on robinhood", ans.Amount)
}

func main() {
	full := &Host{caps: Caps{true, true, true}, approve: true}
	fmt.Println("sampling 1 declared:", full.caps)
	fmt.Println("sampling 5:", summarise(full, 5000, true))
	none := &Host{caps: Caps{Roots: true}}
	fmt.Println("sampling 6 (no check):", summarise(none, 5000, false))
	fmt.Println("sampling 7 (check first):", summarise(none, 5000, true))

	roots := []string{"/home/dev/repo"}
	for _, p := range []string{"/home/dev/repo/README.md", "/home/dev/.ssh/id_ed25519"} {
		fmt.Printf("roots: %s inside=%v\n", p, insideRoots(roots, p))
	}
	roots = append(roots, "/home/dev/docs") // notifications/roots/list_changed
	fmt.Println("roots 4 after list_changed:", roots)
	fmt.Println("roots 5 hostile server reads ~/.ssh: private key (roots are advice)")
	fmt.Println("roots 6 hostile server in a sandbox:", sandboxOpen(roots[:1], "/home/dev/.ssh/id_ed25519"))

	form := []string{"amount:number", "network:string"}
	fmt.Println("elicit 4:", topup(form, Answer{"accept", 10}))
	fmt.Println("elicit 5:", topup(form, Answer{Action: "decline"}))
	fmt.Println("elicit 6:", topup(append(form, "private_key:string"), Answer{"accept", 10}))
}
