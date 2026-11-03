// A dashboard reads fee_bps from the pool service. Replays the lesson
// 13-contract-first: code first (the break ships), then contract first
// (a consumer pact and a breaking-change linter stop it in the build).
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
)

// Schema is the contract: field name -> JSON type.
type Schema map[string]string

// Pact is what one consumer reads: field -> type. Only the fields it uses.
type Pact map[string]string

// poolService answers GET /pools/42 with the given fields.
func poolService(body map[string]any) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /pools/42", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(body)
	})
	return mux
}

func jsonType(v any) string {
	switch v.(type) {
	case float64:
		return "integer"
	case string:
		return "string"
	}
	return "other"
}

// lint diffs the new contract against main: a removed or retyped field breaks.
func lint(old, nw Schema) []string {
	var breaks []string
	for f, t := range old {
		if nt, ok := nw[f]; !ok {
			breaks = append(breaks, f+" removed")
		} else if nt != t {
			breaks = append(breaks, f+" changed "+t+" -> "+nt)
		}
	}
	sort.Strings(breaks)
	return breaks
}

// verify replays a consumer's pact against the real provider.
func verify(p Pact, provider http.Handler, exact bool) []string {
	srv := httptest.NewServer(provider)
	defer srv.Close()
	res, err := http.Get(srv.URL + "/pools/42")
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var got map[string]any
	json.Unmarshal(raw, &got)
	var fails []string
	for f, t := range p {
		if v, ok := got[f]; !ok || jsonType(v) != t {
			fails = append(fails, "needs "+f+" as "+t)
		}
	}
	if exact && len(got) != len(p) { // a greedy pact: the whole body, nothing more
		fails = append(fails, "unexpected extra field")
	}
	sort.Strings(fails)
	return fails
}

// dashboard reads fee_bps as a number.
func dashboard(provider http.Handler) string {
	srv := httptest.NewServer(provider)
	defer srv.Close()
	res, err := http.Get(srv.URL + "/pools/42")
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	var pool struct {
		FeeBps int `json:"fee_bps"`
	}
	if err := json.NewDecoder(res.Body).Decode(&pool); err != nil {
		return "fee broken: " + err.Error()
	}
	return fmt.Sprintf("fee %d", pool.FeeBps)
}

func main() {
	v1 := map[string]any{"id": 42, "fee_bps": 30}
	str := map[string]any{"id": 42, "fee_bps": "30"}
	fixed := map[string]any{"id": 42, "fee_bps": 30, "fee_text": "30"}

	fmt.Println("--- code first, file later")
	fmt.Println("dashboard on v1:", dashboard(poolService(v1)))
	fmt.Println("file regenerated from code: fee_bps string, nothing compares it")
	fmt.Println("provider's own tests: pass, ship")
	fmt.Println("dashboard in production:", dashboard(poolService(str)))

	fmt.Println("--- contract first, consumer tests")
	file := Schema{"id": "integer", "fee_bps": "integer"}
	pact := Pact{"fee_bps": "integer"} // the dashboard reads only this
	fmt.Println("pact on v1:", verify(pact, poolService(v1), false))
	edited := Schema{"id": "integer", "fee_bps": "string"}
	fmt.Println("linter on the change:", lint(file, edited))
	fmt.Println("pact on the change:", verify(pact, poolService(str), false), "-> build red")
	added := Schema{"id": "integer", "fee_bps": "integer", "fee_text": "string"}
	fmt.Println("add fee_text instead: linter", lint(file, added), "pact", verify(pact, poolService(fixed), false), "-> green")
	greedy := Pact{"id": "integer", "fee_bps": "integer"}
	fmt.Println("greedy whole-body pact on the addition:", verify(greedy, poolService(fixed), true))
}
