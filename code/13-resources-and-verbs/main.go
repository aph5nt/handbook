// An analyst app and a link prefetcher call a pool API. Replays the lesson
// 13-resources-and-verbs: an RPC-style API where a GET deletes a label,
// then a resource API where every GET only reads.
package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
)

type DB struct{ labels map[string]bool } // label id -> exists

// rpcAPI names functions in the path. Picked by path only.
func rpcAPI(db *DB) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/getPool", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "pool "+r.URL.Query().Get("id"))
	})
	mux.HandleFunc("/doTransfer", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "moved") // a retry cannot tell if this is safe
	})
	mux.HandleFunc("/deleteLabel", func(w http.ResponseWriter, r *http.Request) {
		delete(db.labels, r.URL.Query().Get("id")) // a GET that writes
		fmt.Fprint(w, "deleted")
	})
	return mux
}

// restAPI names things in the path and picks the handler by method AND path.
func restAPI(db *DB) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /pools/{id}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "pool "+r.PathValue("id")) // GET only reads
	})
	mux.HandleFunc("GET /pools/{id}/swaps", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "swaps of pool "+r.PathValue("id"))
	})
	mux.HandleFunc("GET /labels", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "labels of trader "+r.URL.Query().Get("trader"))
	})
	mux.HandleFunc("POST /transfers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/transfers/77")
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("DELETE /labels/{id}", func(w http.ResponseWriter, r *http.Request) {
		delete(db.labels, r.PathValue("id"))
		w.WriteHeader(http.StatusNoContent)
	})
	return mux // a method with no route gets 405 Method Not Allowed
}

func call(method, url string) string {
	req, _ := http.NewRequest(method, url, nil)
	req.Header.Set("Authorization", "Bearer analyst") // no session: every request carries its token
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	out := fmt.Sprintf("%d %s", res.StatusCode, strings.TrimSpace(string(b)))
	if loc := res.Header.Get("Location"); loc != "" {
		out += " Location " + loc
	}
	return out
}

// prefetch follows every GET link on the page, as a browser prefetcher does.
func prefetch(base string, links []string) {
	for _, l := range links {
		fmt.Println("  prefetcher GET", l, "->", call(http.MethodGet, base+l))
	}
}

func main() {
	db := &DB{labels: map[string]bool{"3": true}}
	srv := httptest.NewServer(rpcAPI(db))
	fmt.Println("--- RPC in a REST coat")
	fmt.Println("GET /getPool?id=42 ->", call("GET", srv.URL+"/getPool?id=42"))
	fmt.Println("POST /doTransfer ->", call("POST", srv.URL+"/doTransfer"))
	prefetch(srv.URL, []string{"/getPool?id=42", "/deleteLabel?id=3"})
	fmt.Println("label 3 exists:", db.labels["3"])
	srv.Close()

	db = &DB{labels: map[string]bool{"3": true}}
	srv = httptest.NewServer(restAPI(db))
	fmt.Println("--- Resources and verbs")
	fmt.Println("GET /pools/42 ->", call("GET", srv.URL+"/pools/42"))
	fmt.Println("GET /pools/42/swaps ->", call("GET", srv.URL+"/pools/42/swaps"))
	fmt.Println("GET /labels?trader=3 ->", call("GET", srv.URL+"/labels?trader=3"))
	fmt.Println("POST /transfers ->", call("POST", srv.URL+"/transfers"))
	fmt.Println("GET /transfers (no GET route) ->", call("GET", srv.URL+"/transfers"))
	prefetch(srv.URL, []string{"/pools/42"})
	fmt.Println("label 3 exists:", db.labels["3"], "(only DELETE /labels/3 removes it)")
	srv.Close()
}
