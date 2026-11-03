// Six changes to a transfers API, and what they do to a strict and to a
// tolerant client. Replays the lesson 13-backward-compatibility-rules.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
)

// strict parses into a fixed struct and switches over kind with no default.
func strict(body []byte) error {
	var t struct {
		Amount *float64 `json:"amount"`
		Kind   string   `json:"kind"`
	}
	if err := json.Unmarshal(body, &t); err != nil {
		return err
	}
	if t.Amount == nil {
		return fmt.Errorf("amount is missing")
	}
	switch t.Kind {
	case "transfer", "swap":
		return nil
	}
	return fmt.Errorf("unknown kind %q", t.Kind) // no default branch: it crashes
}

// tolerant ignores unknown fields and keeps unknown enum values as other.
func tolerant(body []byte) error {
	var t struct {
		Amount *float64 `json:"amount"`
		Kind   string   `json:"kind"`
	}
	if err := json.Unmarshal(body, &t); err != nil {
		return err
	}
	if t.Amount == nil {
		return fmt.Errorf("amount is missing")
	}
	return nil // any kind is fine: unknown shows as "other"
}

func verdict(err error) string {
	if err != nil {
		return "breaks: " + err.Error()
	}
	return "ok"
}

// server has a required parameter and an address rule, both switchable.
func server(requireNetwork, rejectUpper bool) http.Handler {
	lower := regexp.MustCompile(`^0x[0-9a-f]{40}$`)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requireNetwork && r.URL.Query().Get("network") == "" {
			http.Error(w, "network is required", http.StatusBadRequest)
			return
		}
		if a := r.URL.Query().Get("address"); rejectUpper && !lower.MatchString(a) {
			http.Error(w, "address must be lower case", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `{"amount":1,"kind":"transfer"}`)
	})
}

// oldCall is a caller written before either change.
func oldCall(h http.Handler) int {
	srv := httptest.NewServer(h)
	defer srv.Close()
	res, err := http.Get(srv.URL + "/transfers?address=0xABABABABABABABABABABABABABABABABABABABAB")
	if err != nil {
		panic(err)
	}
	res.Body.Close()
	return res.StatusCode
}

// lint is the server half of the rules: add only.
func lint(old, nw map[string]bool) []string {
	var fails []string
	for f := range old {
		if !nw[f] {
			fails = append(fails, "removed "+f)
		}
	}
	return fails
}

func main() {
	bodies := []struct{ name, body string }{
		{"add a field usd_at_day", `{"amount":1,"kind":"transfer","usd_at_day":2}`},
		{"add enum value bridge", `{"amount":1,"kind":"bridge"}`},
		{"rename amount to value", `{"value":1,"kind":"transfer"}`},
	}
	for _, b := range bodies {
		fmt.Printf("%s: strict %s | tolerant %s\n", b.name, verdict(strict([]byte(b.body))), verdict(tolerant([]byte(b.body))))
	}
	fmt.Println("new required input network: old caller gets", oldCall(server(true, false)))
	fmt.Println("reject upper-case address: old caller gets", oldCall(server(false, true)))
	fmt.Println("no change: old caller gets", oldCall(server(false, false)))
	old := map[string]bool{"amount": true, "kind": true}
	fmt.Println("linter on the rename:", lint(old, map[string]bool{"value": true, "kind": true}))
	fmt.Println("linter on the addition:", lint(old, map[string]bool{"amount": true, "kind": true, "usd_at_day": true}))
}
