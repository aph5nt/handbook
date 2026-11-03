// Ending v1 of an API on a date. Replays the lesson 13-deprecation-policy:
// headers on every answer, a usage report per caller, a brownout, and 410
// Gone on the date. Then the same plan where the date keeps slipping.
package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"time"
)

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// V1 wraps the old version with the deprecation policy.
type V1 struct {
	now      func() time.Time
	sunset   time.Time
	brownout [2]time.Time
	usage    map[string]int
}

func (v *V1) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	now := v.now()
	v.usage[r.Header.Get("X-Client")]++
	if !now.Before(v.sunset) {
		w.Header().Set("Link", `</v2/transfers>; rel="successor-version"`)
		http.Error(w, "v1 ended, use v2", http.StatusGone)
		return
	}
	if !now.Before(v.brownout[0]) && now.Before(v.brownout[1]) {
		w.Header().Set("Retry-After", "3600")
		http.Error(w, "planned v1 brownout", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Deprecation", "true")
	w.Header().Set("Sunset", v.sunset.Format(http.TimeFormat))
	w.Header().Set("Link", `</docs/move-to-v2>; rel="deprecation"`)
	fmt.Fprint(w, `[{"amount":1}]`)
}

// report is the weekly list: calls per caller, biggest first.
func (v *V1) report() []string {
	var names []string
	for n := range v.usage {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool { return v.usage[names[i]] > v.usage[names[j]] })
	var out []string
	for _, n := range names {
		out = append(out, fmt.Sprintf("%s %d", n, v.usage[n]))
	}
	return out
}

func call(url, client string) *http.Response {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Client", client)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	res.Body.Close()
	return res
}

const never = 1 << 30

// Caller moves to v2 at month MovesAt, if it still has a plan.
type Caller struct {
	Name    string
	MovesAt int
}

// planEnd walks the months. No report and no calls: a complaint moves the
// date, and every caller who hears it drops its own plan.
func planEnd(callers []*Caller, date, horizon int, gaps []int) []int {
	dates := []int{date}
	for month := 0; month < horizon && len(gaps) > 0; month++ {
		if month != date-1 || moved(callers, date) == len(callers) {
			continue
		}
		date += gaps[0] // not ready: the team moves the date
		gaps = gaps[1:]
		dates = append(dates, date)
		for _, c := range callers {
			c.MovesAt = never // nobody plans for a date that moves
		}
	}
	return dates
}

func moved(callers []*Caller, month int) int {
	n := 0
	for _, c := range callers {
		if c.MovesAt <= month {
			n++
		}
	}
	return n
}

func main() {
	clock := day(2026, 9, 1)
	v := &V1{now: func() time.Time { return clock }, sunset: day(2027, 3, 1), usage: map[string]int{},
		brownout: [2]time.Time{day(2027, 2, 22), day(2027, 2, 22).Add(time.Hour)}}
	srv := httptest.NewServer(v)
	defer srv.Close()

	fmt.Println("--- a dated promise, kept")
	res := call(srv.URL, "big client")
	fmt.Println("headers:", res.Header.Get("Deprecation"), "|", res.Header.Get("Sunset"), "|", res.Header.Get("Link"))
	for i := 0; i < 400; i++ {
		call(srv.URL, "big client")
	}
	for i := 0; i < 20; i++ {
		call(srv.URL, "small client")
	}
	fmt.Println("week 1 report:", v.report())
	v.usage = map[string]int{} // the small client moved to v2
	clock = day(2027, 2, 15)
	for i := 0; i < 400; i++ {
		call(srv.URL, "big client")
	}
	fmt.Println("two weeks before the date:", v.report(), "-> call them")
	clock = day(2027, 2, 22).Add(10 * time.Minute)
	res = call(srv.URL, "forgotten script")
	fmt.Println("brownout:", res.StatusCode, "Retry-After", res.Header.Get("Retry-After"))
	clock = day(2027, 2, 22).Add(2 * time.Hour)
	fmt.Println("after the brownout:", call(srv.URL, "forgotten script").StatusCode)
	clock = day(2027, 3, 1)
	res = call(srv.URL, "forgotten script")
	fmt.Println("on the date:", res.StatusCode, res.Header.Get("Link"))

	fmt.Println("--- a date that slips")
	callers := []*Caller{{"small client", 5}, {"big client", never}}
	dates := planEnd(callers, 6, 24, []int{3, 6, 6, 6})
	fmt.Println("end date, in months from the announcement:", dates)
	fmt.Println("month 24: callers moved:", moved(callers, 24), "| live versions 2, every bug fix done 2 times")
}
