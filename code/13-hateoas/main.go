// A subscription API that hands out its allowed actions as links. Replays
// the lesson 13-hateoas: an app with a copied rule offers a button the
// server refuses, an app that draws links needs no release.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
)

// Sub is subscription 7.
type Sub struct {
	Status string
	Unpaid bool
}

// Link names one allowed action.
type Link struct {
	Href   string `json:"href"`
	Method string `json:"method"`
}

// API holds the state and the one copy of the rule.
type API struct {
	sub Sub
	// rule is finance's new rule, off at the start of the story.
	rule bool
}

func (a *API) links() map[string]Link {
	out := map[string]Link{}
	if a.sub.Status == "active" {
		out["topup"] = Link{"/subscriptions/7/topup", "POST"}
		if !(a.rule && a.sub.Unpaid) {
			out["cancel"] = Link{"/subscriptions/7/cancel", "POST"}
		}
	}
	if a.sub.Status == "cancelled" {
		out["reactivate"] = Link{"/subscriptions/7/reactivate", "POST"}
	}
	return out
}

func (a *API) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /subscriptions/7", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"status": a.sub.Status, "_links": a.links()})
	})
	mux.HandleFunc("POST /subscriptions/7/cancel", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := a.links()["cancel"]; !ok {
			http.Error(w, "unpaid invoice", http.StatusConflict)
			return
		}
		a.sub.Status = "cancelled"
		json.NewEncoder(w).Encode(map[string]any{"status": a.sub.Status, "_links": a.links()})
	})
	return mux
}

type answer struct {
	Status string          `json:"status"`
	Links  map[string]Link `json:"_links"`
}

func read(res *http.Response, err error) answer {
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	var a answer
	json.NewDecoder(res.Body).Decode(&a)
	return a
}

// hardcodedButtons is the app's own copy of the rule.
func hardcodedButtons(a answer) []string {
	if a.Status == "active" {
		return []string{"Top up", "Cancel"}
	}
	return nil
}

// buttonsFromLinks draws one button per link name the app knows.
func buttonsFromLinks(a answer) []string {
	known := map[string]string{"topup": "Top up", "cancel": "Cancel", "reactivate": "Reactivate"}
	var out []string
	for name := range a.Links {
		if label, ok := known[name]; ok {
			out = append(out, label)
		}
	}
	sort.Strings(out)
	return out
}

func main() {
	a := &API{sub: Sub{Status: "active"}}
	srv := httptest.NewServer(a.routes())
	defer srv.Close()
	get := func() answer { return read(http.Get(srv.URL + "/subscriptions/7")) }

	fmt.Println("--- rules copied into the app")
	fmt.Println("app shows:", hardcodedButtons(get()))
	a.rule, a.sub.Unpaid = true, true
	fmt.Println("rule on, invoice unpaid, app still shows:", hardcodedButtons(get()))
	res, err := http.Post(srv.URL+"/subscriptions/7/cancel", "", nil)
	if err != nil {
		panic(err)
	}
	res.Body.Close()
	fmt.Println("user taps Cancel:", res.StatusCode)

	fmt.Println("--- buttons from links")
	a.rule, a.sub.Unpaid = false, false
	fmt.Println("before the rule:", buttonsFromLinks(get()))
	a.rule, a.sub.Unpaid = true, true
	fmt.Println("rule on, invoice unpaid:", buttonsFromLinks(get()), "(no app release)")
	a.sub.Unpaid = false
	now := get()
	fmt.Println("invoice paid:", buttonsFromLinks(now))
	after := read(http.Post(srv.URL+now.Links["cancel"].Href, "", nil))
	fmt.Println("followed cancel:", after.Status, buttonsFromLinks(after))

	fmt.Println("--- where it is not worth it")
	plain, _ := json.Marshal([]string{"swap-1", "swap-2"})
	linked, _ := json.Marshal(map[string]any{"swaps": []string{"swap-1", "swap-2"}, "_links": map[string]string{"self": "/swaps"}})
	fmt.Printf("list of swaps: %d bytes, with links %d bytes, nobody follows them\n", len(plain), len(linked))
}
