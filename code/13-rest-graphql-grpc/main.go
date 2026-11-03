// One dashboard, three API styles. Replays the lesson 13-rest-graphql-grpc.
// REST and the cache use net/http. GraphQL is modelled with plain resolver
// functions (no GraphQL library: standard library only). gRPC is modelled
// as a server stream over HTTP chunks (no gRPC library): one request, many
// typed messages, flushed one by one.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
)

// ---- REST: one URL per resource, an HTTP cache in front

var dbReads int

func restAPI() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /addresses/{a}", func(w http.ResponseWriter, r *http.Request) {
		dbReads++
		fmt.Fprintf(w, `{"address":%q}`, r.PathValue("a"))
	})
	mux.HandleFunc("GET /addresses/{a}/labels", func(w http.ResponseWriter, r *http.Request) {
		dbReads++
		fmt.Fprint(w, `["Exchange"]`)
	})
	mux.HandleFunc("GET /addresses/{a}/transfers", func(w http.ResponseWriter, r *http.Request) {
		dbReads++
		row := map[string]int{}
		for f := 0; f < 40; f++ { // 40 fields per transfer, one size for all
			row[fmt.Sprintf("f%02d", f)] = f
		}
		json.NewEncoder(w).Encode([]map[string]int{row})
	})
	return mux
}

// cache stores each GET answer by URL: the next user never reaches the API.
func cache(next http.Handler) http.Handler {
	saved := map[string]string{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if body, ok := saved[r.URL.String()]; ok {
			fmt.Fprint(w, body)
			return
		}
		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, r)
		saved[r.URL.String()] = rec.Body.String()
		fmt.Fprint(w, rec.Body.String())
	})
}

// ---- GraphQL: a list resolver and a field resolver per item

var sqlQueries int

func selectAddresses(n int) []string {
	sqlQueries++
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("0x%02x", i)
	}
	return out
}

func selectLabels(keys []string) map[string][]string {
	sqlQueries++ // one query, however many keys
	out := map[string][]string{}
	for _, k := range keys {
		out[k] = []string{"Exchange"}
	}
	return out
}

// resolve runs addresses { labels }. With a loader, labels are batched.
func resolve(n int, loader bool) map[string][]string {
	addrs := selectAddresses(n)
	res := map[string][]string{}
	if !loader {
		for _, a := range addrs { // N+1: one query per address
			res[a] = selectLabels([]string{a})[a]
		}
		return res
	}
	var keys []string
	for _, a := range addrs { // each resolver only hands its key over
		keys = append(keys, a)
	}
	return selectLabels(keys) // the loader's one batch
}

// admit refuses a query before it runs when it is too deep.
func admit(query string, maxDepth int) error {
	depth, top := 0, 0
	for _, c := range query {
		if c == '{' {
			depth++
			top = max(top, depth)
		} else if c == '}' {
			depth--
		}
	}
	if top > maxDepth {
		return fmt.Errorf("refused: depth %d over %d", top, maxDepth)
	}
	return nil
}

// ---- gRPC stand-in: StreamBlocks(from) sends many typed messages

type Block struct {
	Height int    `json:"height"`
	Hash   string `json:"hash"`
}

func streamBlocks(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "application/grpc" {
		http.Error(w, "raw gRPC only: use a gRPC-Web proxy", http.StatusUnsupportedMediaType)
		return
	}
	enc, f := json.NewEncoder(w), w.(http.Flusher)
	for h := 100; h <= 102; h++ {
		enc.Encode(Block{h, fmt.Sprintf("0x%x", h*7919)})
		f.Flush() // each message leaves now, on the same call
	}
}

// grpcWebProxy is the Envoy role: it turns a browser call into a gRPC call.
func grpcWebProxy(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") == "application/grpc-web" {
			r.Header.Set("Content-Type", "application/grpc")
		}
		next(w, r)
	}
}

func openStream(url, ctype string) (int, []Block) {
	req, _ := http.NewRequest("POST", url, nil)
	req.Header.Set("Content-Type", ctype)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	var got []Block
	sc := bufio.NewScanner(res.Body)
	for sc.Scan() {
		var b Block
		if json.Unmarshal(sc.Bytes(), &b) == nil {
			got = append(got, b)
		}
	}
	return res.StatusCode, got
}

func main() {
	fmt.Println("--- REST: three calls")
	srv := httptest.NewServer(cache(restAPI()))
	paths := []string{"/addresses/0xab", "/addresses/0xab/labels", "/addresses/0xab/transfers?limit=10"}
	for user := 1; user <= 2; user++ {
		size := 0
		for _, p := range paths {
			res, err := http.Get(srv.URL + p)
			if err != nil {
				panic(err)
			}
			var b strings.Builder
			bufio.NewReader(res.Body).WriteTo(&b)
			res.Body.Close()
			size += b.Len()
		}
		fmt.Printf("user %d: %d round trips, %d bytes, database reads so far %d\n", user, len(paths), size, dbReads)
	}
	fmt.Println("transfer fields sent 40, dashboard shows 4")
	srv.Close()

	fmt.Println("--- GraphQL: N+1 and DataLoader")
	resolve(100, false)
	fmt.Println("without DataLoader: sql queries", sqlQueries)
	sqlQueries = 0
	resolve(100, true)
	fmt.Println("with DataLoader: sql queries", sqlQueries)
	deep := "{ transfers { sender { transfers { sender { transfers { sender { transfers { sender { transfers { hash } } } } } } } } } }"
	fmt.Println("nested query:", admit(deep, 5))

	fmt.Println("--- gRPC: a typed stream (HTTP chunk stand-in)")
	g := httptest.NewServer(grpcWebProxy(streamBlocks))
	defer g.Close()
	code, blocks := openStream(g.URL+"/StreamBlocks", "application/grpc")
	fmt.Println("sync service:", code, blocks)
	code, _ = openStream(g.URL+"/StreamBlocks", "text/plain")
	fmt.Println("browser, no proxy:", code)
	code, blocks = openStream(g.URL+"/StreamBlocks", "application/grpc-web")
	fmt.Println("browser via gRPC-Web proxy:", code, len(blocks), "blocks")
}
