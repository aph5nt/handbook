// A report sums transfer values in USD. Replays the lesson 13-versioning:
// a field that quietly changes meaning, then add-only changes, a new
// version only for a real break, a sunset date and 410 Gone after it.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"time"
)

type transfer struct {
	Amount float64 // tokens moved
	Day    float64 // token price at the day close
	Minute float64 // token price at the minute of the transfer
}

var transfers = []transfer{{100, 2.00, 2.30}, {50, 2.00, 1.80}, {10, 4.00, 4.60}}

// API serves /transfers and /v2/transfers. sunset ends v1.
type API struct {
	minuteSource bool // the quiet change
	addMinute    bool // the add-only change
	sunset       time.Time
	now          func() time.Time
	calls        map[string]map[string]bool // version -> client names
}

func (a *API) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /transfers", a.v1)
	mux.HandleFunc("GET /v2/transfers", a.v2)
	return mux
}

func (a *API) v1(w http.ResponseWriter, r *http.Request) {
	a.seen("v1", r.Header.Get("X-Client"))
	if !a.sunset.IsZero() {
		if !a.now().Before(a.sunset) {
			w.Header().Set("Link", `</v2/transfers>; rel="successor-version"`)
			http.Error(w, "v1 is gone, use /v2/transfers", http.StatusGone)
			return
		}
		w.Header().Set("Deprecation", "true")
		w.Header().Set("Sunset", a.sunset.Format(http.TimeFormat))
	}
	var out []map[string]any
	for _, t := range transfers {
		usd := t.Amount * t.Day
		if a.minuteSource {
			usd = t.Amount * t.Minute // same name, new meaning: the quiet change
		}
		row := map[string]any{"amount": t.Amount, "token": "WETH", "usd_at_day": usd}
		if a.addMinute {
			row["usd_at_minute"] = t.Amount * t.Minute // a new meaning gets a new name
		}
		out = append(out, row)
	}
	json.NewEncoder(w).Encode(out)
}

// v2 changes an existing field (amount becomes a decimal string): a real break.
func (a *API) v2(w http.ResponseWriter, r *http.Request) {
	a.seen("v2", r.Header.Get("X-Client"))
	var out []map[string]any
	for _, t := range transfers {
		out = append(out, map[string]any{"amount": fmt.Sprintf("%.18f", t.Amount), "usd_at_day": t.Amount * t.Day})
	}
	json.NewEncoder(w).Encode(out)
}

func (a *API) seen(v, client string) {
	if a.calls[v] == nil {
		a.calls[v] = map[string]bool{}
	}
	a.calls[v][client] = true
}

// report is the old client: it sums usd_at_day and ignores unknown fields.
func report(url, client string) (float64, *http.Response) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Client", client)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	var rows []struct {
		USD float64 `json:"usd_at_day"`
	}
	json.NewDecoder(res.Body).Decode(&rows)
	sum := 0.0
	for _, r := range rows {
		sum += r.USD
	}
	return sum, res
}

func main() {
	books := 0.0 // finance's books at the day price
	for _, t := range transfers {
		books += t.Amount * t.Day
	}

	fmt.Println("--- quiet change of meaning")
	a := &API{now: time.Now, calls: map[string]map[string]bool{}}
	srv := httptest.NewServer(a.routes())
	sum, res := report(srv.URL+"/transfers", "report")
	fmt.Printf("report %.2f books %.2f status %d\n", sum, books, res.StatusCode)
	a.minuteSource = true
	sum, res = report(srv.URL+"/transfers", "report")
	fmt.Printf("after the price source change: report %.2f books %.2f status %d, no error\n", sum, books, res.StatusCode)
	srv.Close()

	fmt.Println("--- add, then version with a sunset")
	clock := time.Date(2027, 1, 10, 0, 0, 0, 0, time.UTC)
	b := &API{now: func() time.Time { return clock }, addMinute: true, calls: map[string]map[string]bool{}}
	srv = httptest.NewServer(b.routes())
	defer srv.Close()
	sum, _ = report(srv.URL+"/transfers", "report")
	fmt.Printf("usd_at_minute added beside usd_at_day: report %.2f books %.2f\n", sum, books)
	b.sunset = time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC)
	for _, c := range []string{"report", "tax-export", "wallet-ui"} {
		_, res = report(srv.URL+"/transfers", c)
	}
	fmt.Println("v1 headers: Deprecation", res.Header.Get("Deprecation"), "Sunset", res.Header.Get("Sunset"))
	var names []string
	for c := range b.calls["v1"] {
		names = append(names, c)
	}
	sort.Strings(names)
	fmt.Println("clients still on v1:", names)
	clock = b.sunset
	_, res = report(srv.URL+"/transfers", "report")
	fmt.Println("on the sunset date:", res.StatusCode, "Link", res.Header.Get("Link"))
	_, res = report(srv.URL+"/v2/transfers", "report")
	fmt.Println("v2:", res.StatusCode)

	fmt.Println("--- a version per change")
	versions := []string{"v1", "v2", "v3", "v4", "v5"}
	fmt.Printf("one bug fix touches %d live versions\n", len(versions))
}
