// A phone tops up a wallet on a train. Replays the lesson 13-idempotency-key:
// a lost answer and a retry charge twice; a key saved in one transaction with
// the money does not; a key used for another body is refused; and a key saved
// outside that transaction loses it all in a crash.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
)

// Stored is what the key remembers: the request and its answer.
type Stored struct {
	Body   string
	Status int
	Answer string
}

// API holds the ledger and the key store in one database.
type API struct {
	mu        sync.Mutex // the database transaction
	ledger    int
	keys      map[string]*Stored
	useKeys   bool
	keyApart  bool // the key lives in a cache, away from the money
	crashOnce bool // the process dies after the money moved
}

func (a *API) topup(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var req struct {
		USD int `json:"usd"`
	}
	json.Unmarshal(raw, &req)
	a.mu.Lock() // lock the key: a second copy of this request waits here
	defer a.mu.Unlock()
	key := r.Header.Get("Idempotency-Key")
	if a.useKeys && key != "" {
		if s, ok := a.keys[key]; ok {
			if s.Body != string(raw) {
				http.Error(w, "this key was used for another request", http.StatusUnprocessableEntity)
				return
			}
			w.WriteHeader(s.Status) // the stored answer: the ledger is not touched
			fmt.Fprint(w, s.Answer)
			return
		}
	}
	a.ledger += req.USD
	answer := fmt.Sprintf(`{"balance":%d}`, a.ledger)
	if a.keyApart && a.crashOnce {
		a.crashOnce = false // money committed, the key never reached the cache
		panic(http.ErrAbortHandler)
	}
	if a.useKeys && key != "" {
		a.keys[key] = &Stored{string(raw), http.StatusCreated, answer} // same transaction
	}
	w.WriteHeader(http.StatusCreated)
	fmt.Fprint(w, answer)
}

// send posts a top-up and returns the status. A crash reads as no answer.
func send(url, key string, usd int) (int, string) {
	body, _ := json.Marshal(map[string]int{"usd": usd})
	req, _ := http.NewRequest("POST", url+"/topups", bytes.NewReader(body))
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "no answer"
	}
	defer res.Body.Close()
	answer, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(bytes.TrimSpace(answer))
}

func start(a *API) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /topups", a.topup)
	return httptest.NewServer(mux)
}

func main() {
	fmt.Println("--- without a key")
	a := &API{keys: map[string]*Stored{}}
	srv := start(a)
	send(srv.URL, "", 10) // the answer is lost in the tunnel
	send(srv.URL, "", 10) // the app retries
	fmt.Println("user paid once, ledger holds", a.ledger, "USD")
	srv.Close()

	fmt.Println("--- with an Idempotency-Key")
	b := &API{keys: map[string]*Stored{}, useKeys: true}
	srv = start(b)
	defer srv.Close()
	code, answer := send(srv.URL, "k1", 10)
	fmt.Println("first, then the answer is lost:", code, answer, "| ledger", b.ledger)
	code, answer = send(srv.URL, "k1", 10)
	fmt.Println("retry with k1:", code, answer, "| ledger", b.ledger)
	code, answer = send(srv.URL, "k1", 50)
	fmt.Println("k1 with 50 USD:", code, answer, "| ledger", b.ledger)

	fmt.Println("--- the key in a cache, the money in the database")
	c := &API{keys: map[string]*Stored{}, useKeys: true, keyApart: true, crashOnce: true}
	s3 := start(c)
	defer s3.Close()
	code, answer = send(s3.URL, "k2", 10)
	fmt.Println("first:", code, answer, "| ledger", c.ledger, "| key saved:", c.keys["k2"] != nil)
	code, answer = send(s3.URL, "k2", 10)
	fmt.Println("retry with k2:", code, answer, "| ledger", c.ledger)
}
