// Paging swaps by cursor. Replays the lesson 13-cursor-design: a cursor on
// block alone skips or repeats rows, an opaque signed cursor with a
// tie-breaker does neither and refuses a token used with other filters.
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
)

type Swap struct {
	Block int `json:"block"`
	Log   int `json:"log"`
}

// swaps holds block 9505 log 0, block 9506 logs 0 to 6, block 9507 log 0.
var swaps = func() []Swap {
	s := []Swap{{9505, 0}}
	for l := 0; l <= 6; l++ {
		s = append(s, Swap{9506, l})
	}
	return append(s, Swap{9507, 0})
}()

func where(match func(Swap) bool, limit int) []Swap {
	var out []Swap
	for _, s := range swaps {
		if match(s) && len(out) < limit {
			out = append(out, s)
		}
	}
	return out
}

// ---- the opaque cursor

const sortVersion = 1

var key = []byte("server-only")

type token struct {
	Block, Log, Sort int
	Pool             string
}

func encode(t token) string {
	raw, _ := json.Marshal(t)
	mac := hmac.New(sha256.New, key)
	mac.Write(raw)
	return base64.URLEncoding.EncodeToString(append(mac.Sum(nil), raw...))
}

func decode(s string) (token, error) {
	var t token
	all, err := base64.URLEncoding.DecodeString(s)
	if err != nil || len(all) < sha256.Size {
		return t, fmt.Errorf("bad cursor")
	}
	sig, raw := all[:sha256.Size], all[sha256.Size:]
	mac := hmac.New(sha256.New, key)
	mac.Write(raw)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return t, fmt.Errorf("cursor signature does not match")
	}
	return t, json.Unmarshal(raw, &t)
}

func list(version int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool := r.URL.Query().Get("pool")
		after := token{Block: -1, Log: -1, Sort: version, Pool: pool}
		if c := r.URL.Query().Get("cursor"); c != "" {
			t, err := decode(c)
			switch {
			case err != nil:
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			case t.Pool != pool:
				http.Error(w, "cursor was made for other filters", http.StatusBadRequest)
				return
			case t.Sort != version:
				http.Error(w, "cursor is from an old sort: start again", http.StatusBadRequest)
				return
			}
			after = t
		}
		rows := where(func(s Swap) bool {
			return s.Block > after.Block || (s.Block == after.Block && s.Log > after.Log)
		}, 4)
		var next *string
		if len(rows) == 4 {
			last := rows[3]
			c := encode(token{last.Block, last.Log, version, pool})
			next = &c
		}
		json.NewEncoder(w).Encode(map[string]any{"rows": rows, "next_cursor": next})
	}
}

type page struct {
	Rows []Swap  `json:"rows"`
	Next *string `json:"next_cursor"`
}

func get(url string) (int, page) {
	res, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	var p page
	json.NewDecoder(res.Body).Decode(&p)
	return res.StatusCode, p
}

func main() {
	fmt.Println("--- cursor on block only")
	p1 := where(func(s Swap) bool { return true }, 4)
	fmt.Println("page 1:", p1, "cursor: block=9506")
	fmt.Println("block > 9506:", where(func(s Swap) bool { return s.Block > 9506 }, 4), "(logs 3 to 6 are skipped)")
	fmt.Println("block >= 9506:", where(func(s Swap) bool { return s.Block >= 9506 }, 4), "(logs 0 to 2 repeat)")

	fmt.Println("--- opaque cursor with a tie-breaker")
	srv := httptest.NewServer(list(sortVersion))
	defer srv.Close()
	_, a := get(srv.URL + "?pool=42")
	fmt.Println("page 1:", a.Rows)
	raw, _ := base64.URLEncoding.DecodeString(*a.Next)
	fmt.Println("token holds, after the signature:", string(raw[sha256.Size:]))
	_, b := get(srv.URL + "?pool=42&cursor=" + *a.Next)
	fmt.Println("page 2:", b.Rows)
	code, _ := get(srv.URL + "?pool=77&cursor=" + *a.Next)
	fmt.Println("cursor with another pool:", code)
	forged := base64.URLEncoding.EncodeToString(append(make([]byte, sha256.Size), `{"Block":9000,"Pool":"42","Sort":1}`...))
	code, _ = get(srv.URL + "?pool=42&cursor=" + forged)
	fmt.Println("hand-built cursor:", code)
	newer := httptest.NewServer(list(sortVersion + 1))
	defer newer.Close()
	code, _ = get(newer.URL + "?pool=42&cursor=" + *a.Next)
	fmt.Println("old token after the sort changed:", code)
	_, c := get(srv.URL + "?pool=42&cursor=" + *b.Next)
	fmt.Println("last page:", c.Rows, "next_cursor nil:", c.Next == nil)
}
