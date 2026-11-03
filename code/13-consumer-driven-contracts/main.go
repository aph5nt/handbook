// Two consumers publish what they read; the pool service replays it in its
// build. Replays the lesson 13-consumer-driven-contracts. The broker is a
// map; a real one is a server such as Pact Broker.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
)

// Pact is one consumer's record of what it reads. Exact, when set, is a
// copy of the whole body: the greedy kind.
type Pact struct {
	Consumer string
	Fields   map[string]string // field -> type
	Exact    map[string]any
}

func poolService(body map[string]any) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(body)
	})
}

func typeOf(v any) string {
	switch v.(type) {
	case float64:
		return "number"
	case string:
		return "string"
	}
	return "other"
}

// replay runs one pact against the real provider.
func replay(p Pact, provider http.Handler) []string {
	srv := httptest.NewServer(provider)
	defer srv.Close()
	res, err := http.Get(srv.URL)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var got map[string]any
	json.Unmarshal(raw, &got)
	var fails []string
	for f, want := range p.Fields {
		if v, ok := got[f]; !ok || typeOf(v) != want {
			fails = append(fails, p.Consumer+" needs "+f+" as "+want)
		}
	}
	if p.Exact != nil && !reflect.DeepEqual(p.Exact, got) {
		fails = append(fails, p.Consumer+" expected the exact old body")
	}
	sort.Strings(fails)
	return fails
}

// canIDeploy asks the broker: does this version pass every live pact?
func canIDeploy(broker []Pact, provider http.Handler) (bool, []string) {
	var fails []string
	for _, p := range broker {
		fails = append(fails, replay(p, provider)...)
	}
	return len(fails) == 0, fails
}

func main() {
	v1 := map[string]any{"id": 42.0, "fee_bps": 30.0, "tvl_usd": 900.0}
	dashboard := Pact{Consumer: "dashboard", Fields: map[string]string{"fee_bps": "number"}}
	alerts := Pact{Consumer: "alert job", Fields: map[string]string{"tvl_usd": "number"}}
	broker := []Pact{dashboard, alerts}

	fmt.Println("--- pacts that name what is read")
	ok, fails := canIDeploy(broker, poolService(v1))
	fmt.Println("replay on v1:", ok, fails)
	dropped := map[string]any{"id": 42.0, "fee_bps": 30.0}
	ok, fails = canIDeploy(broker, poolService(dropped))
	fmt.Println("tvl_usd removed:", ok, fails)
	added := map[string]any{"id": 42.0, "fee_bps": 30.0, "tvl_usd": 900.0, "fee_tier": "low"}
	ok, fails = canIDeploy(broker, poolService(added))
	fmt.Println("can I deploy the addition:", ok, fails)

	fmt.Println("--- a greedy pact")
	greedy := Pact{Consumer: "dashboard", Exact: v1}
	fmt.Println("v1:", replay(greedy, poolService(v1)))
	fmt.Println("fee_tier added:", replay(greedy, poolService(added)))
	broken := map[string]any{"id": 42.0, "fee_bps": "30", "tvl_usd": 900.0}
	fmt.Println("fix, fields by type, on the addition:", replay(dashboard, poolService(added)))
	fmt.Println("fix, fields by type, on a real break:", replay(dashboard, poolService(broken)))
	moved := map[string]any{"id": 42.0, "fee_bps": 45.0, "tvl_usd": 900.0}
	fmt.Println("fix, a new value of the same type:", replay(dashboard, poolService(moved)))
}
