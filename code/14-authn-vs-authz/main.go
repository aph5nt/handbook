// Authentication versus authorization, replayed on a toy in-process API.
// Story: a graph validator reads metrics (200), tries to change a lane (403),
// a call with no token gets 401. Then analyst Ana edits a case id in the URL:
// the broken handler checks only the token, the fixed one filters by tenant.
package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
)

// Who a token names, and the facts the policy needs.
type identity struct {
	name, role, tenant string
}

// The token store stands in for a signature check: a known token is valid.
var tokens = map[string]identity{
	"tok-validator": {name: "graph-validator", role: "metrics:read"},
	"tok-ana":       {name: "ana", role: "analyst", tenant: "A"},
}

// Cases belong to tenants.
var cases = map[string]string{"101": "A", "202": "B"}

// AuthN: who are you? No valid token means 401.
func authenticate(r *http.Request) (identity, bool) {
	tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	id, ok := tokens[tok]
	return id, ok
}

// AuthZ: may this identity do this action? A role lists its actions.
func allowed(id identity, action string) bool {
	return id.role == action
}

func lanes(w http.ResponseWriter, r *http.Request) {
	id, ok := authenticate(r)
	if !ok {
		http.Error(w, "prove who you are", http.StatusUnauthorized) // 401
		return
	}
	action := "metrics:read"
	if r.Method == http.MethodPut {
		action = "lanes:write"
	}
	if !allowed(id, action) {
		http.Error(w, "known, but no", http.StatusForbidden) // 403
		return
	}
	fmt.Fprint(w, "metrics rows")
}

// Broken: checks the token, then trusts the id in the URL.
func casesBroken(w http.ResponseWriter, r *http.Request) {
	if _, ok := authenticate(r); !ok {
		http.Error(w, "prove who you are", http.StatusUnauthorized)
		return
	}
	caseID := strings.TrimPrefix(r.URL.Path, "/cases/")
	fmt.Fprintf(w, "case %s of tenant %s", caseID, cases[caseID]) // no AuthZ
}

// Fixed: the tenant comes from the token, never from the request.
func casesFixed(w http.ResponseWriter, r *http.Request) {
	id, ok := authenticate(r)
	if !ok {
		http.Error(w, "prove who you are", http.StatusUnauthorized)
		return
	}
	caseID := strings.TrimPrefix(r.URL.Path, "/cases/")
	if cases[caseID] != id.tenant { // WHERE case = ? AND tenant = token.tenant
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	fmt.Fprintf(w, "case %s of tenant %s", caseID, cases[caseID])
}

func call(h http.HandlerFunc, method, path, token string) (int, string) {
	r := httptest.NewRequest(method, path, nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h(w, r)
	return w.Code, strings.TrimSpace(w.Body.String())
}

func main() {
	fmt.Println("Tab 1: two checks")
	for _, c := range []struct{ method, path, token string }{
		{"GET", "/metrics", "tok-validator"},
		{"PUT", "/lanes/b", "tok-validator"},
		{"PUT", "/lanes/b", ""},
	} {
		code, body := call(lanes, c.method, c.path, c.token)
		fmt.Printf("  %-4s %-9s token=%-13q -> %d %s\n", c.method, c.path, c.token, code, body)
	}
	fmt.Println("Tab 2: Ana (tenant A) edits the case id")
	for _, p := range []string{"/cases/101", "/cases/202"} {
		code, body := call(casesBroken, "GET", p, "tok-ana")
		fmt.Printf("  broken %-10s -> %d %s\n", p, code, body)
	}
	code, body := call(casesFixed, "GET", "/cases/202", "tok-ana")
	fmt.Printf("  fixed  %-10s -> %d %s\n", "/cases/202", code, body)
}
