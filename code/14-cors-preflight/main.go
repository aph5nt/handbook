// CORS preflight on 127.0.0.1. A toy browser asks with OPTIONS first, then
// sends the real PUT. The broken gateway demands a token on OPTIONS too,
// and a browser never sends one there.
package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
)

const partner = "https://partner.example"

// api answers the preflight with the exact allow list, without asking for a
// token, and keeps authentication on the real request.
func api(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		if r.Header.Get("Origin") == partner {
			w.Header().Set("Access-Control-Allow-Origin", partner)
			w.Header().Set("Access-Control-Allow-Methods", "PUT")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization")
			w.Header().Set("Access-Control-Max-Age", "600")
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Header.Get("Authorization") != "Bearer tok-1" {
		http.Error(w, "no token", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", partner)
	fmt.Fprint(w, "label saved")
}

// gatewayBroken demands a token on every request, OPTIONS included.
func gatewayBroken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "no token", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// browser is the toy browser's rule: preflight unless a cached answer is
// younger than max age. It never puts a token on the OPTIONS.
type browser struct {
	cachedUntil int
	preflights  int
}

func (b *browser) put(url string, now int) string {
	if now >= b.cachedUntil {
		b.preflights++
		req, _ := http.NewRequest(http.MethodOptions, url, nil)
		req.Header.Set("Origin", partner)
		req.Header.Set("Access-Control-Request-Method", "PUT")
		req.Header.Set("Access-Control-Request-Headers", "Authorization")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err.Error()
		}
		resp.Body.Close()
		ok := resp.StatusCode/100 == 2 && strings.Contains(resp.Header.Get("Access-Control-Allow-Methods"), "PUT")
		if !ok {
			return fmt.Sprintf("CORS error: preflight got %d, PUT never sent", resp.StatusCode)
		}
		b.cachedUntil = now + 600
	}
	req, _ := http.NewRequest(http.MethodPut, url, nil)
	req.Header.Set("Origin", partner)
	req.Header.Set("Authorization", "Bearer tok-1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err.Error()
	}
	defer resp.Body.Close()
	return fmt.Sprintf("PUT %d, page may read: %v", resp.StatusCode, resp.Header.Get("Access-Control-Allow-Origin") == partner)
}

func main() {
	good := httptest.NewServer(http.HandlerFunc(api))
	defer good.Close()
	bad := httptest.NewServer(gatewayBroken(api))
	defer bad.Close()
	b := &browser{}
	fmt.Println("  first PUT             ->", b.put(good.URL, 0), " preflights:", b.preflights)
	fmt.Println("  PUT at second 300     ->", b.put(good.URL, 300), " preflights:", b.preflights)
	fmt.Println("  PUT at second 601     ->", b.put(good.URL, 601), " preflights:", b.preflights)
	fmt.Println("  broken gateway        ->", (&browser{}).put(bad.URL, 0))
	fmt.Println("  fixed, OPTIONS passes ->", (&browser{}).put(good.URL, 0))
}
