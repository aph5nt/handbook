// An OWASP review of one toy export endpoint, on 127.0.0.1. Each finding
// shows the vulnerable handler and the fixed one, and the program proves it.
package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
)

type caseRow struct{ id, tenant, body string }

var cases = []caseRow{{"101", "A", "tenant A rows"}, {"202", "B", "tenant B rows"}}
var tokens = map[string]string{"tok-a": "A"} // token -> the caller's tenant
var audit []string

// exportBroken finds the case by id and never compares tenants: A01.
func exportBroken(w http.ResponseWriter, r *http.Request) {
	for _, c := range cases {
		if c.id == r.URL.Query().Get("id") {
			fmt.Fprint(w, c.body)
			return
		}
	}
	http.NotFound(w, r)
}

// exportFixed takes the tenant from the token, and writes an audit line: A09.
func exportFixed(w http.ResponseWriter, r *http.Request) {
	tenant := tokens[r.Header.Get("Authorization")]
	id := r.URL.Query().Get("id")
	for _, c := range cases {
		if c.id == id && c.tenant == tenant {
			audit = append(audit, fmt.Sprintf("tenant %s exported case %s", tenant, id))
			fmt.Fprint(w, c.body)
			return
		}
	}
	http.NotFound(w, r)
}

// filterParam is A03: the filter is compared as a value, never parsed.
func filterParam(value string) int {
	n := 0
	for _, c := range cases {
		if c.body == value {
			n++
		}
	}
	return n
}

// callbackAllowed is the A10 fix: resolve first, then refuse private ranges.
func callbackAllowed(ip string) bool {
	a := netip.MustParseAddr(ip)
	return !(a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast())
}

func get(h http.HandlerFunc, id string) string {
	s := httptest.NewServer(h)
	defer s.Close()
	req, _ := http.NewRequest("GET", s.URL+"/export?id="+id, nil)
	req.Header.Set("Authorization", "tok-a")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err.Error()
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return fmt.Sprintf("%d %s", resp.StatusCode, strings.TrimSpace(string(b)))
}

func main() {
	fmt.Println("Analyst of tenant A, token tok-a")
	fmt.Println("  A01 broken, case 202 ->", get(exportBroken, "202"), " audit lines:", len(audit))
	fmt.Println("  A01 fixed,  case 202 ->", get(exportFixed, "202"))
	fmt.Println("  A01 fixed,  case 101 ->", get(exportFixed, "101"))
	fmt.Println("  A03 filter x' OR 1=1 -- ->", filterParam("x' OR 1=1 --"), "rows")
	fmt.Println("  A09 audit:", audit)
	fmt.Println("  A10 callback 10.0.0.5 allowed?", callbackAllowed("10.0.0.5"))
	fmt.Println("  A10 callback 93.184.215.14 allowed?", callbackAllowed("93.184.215.14"))
}
