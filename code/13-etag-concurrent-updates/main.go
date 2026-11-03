// Two agents edit watch list 7. Replays both tabs of the lesson
// 13-etag-concurrent-updates: last write wins, then If-Match with an ETag,
// then the timestamp-ETag failure.
package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
)

type Store struct {
	mu        sync.Mutex
	addrs     string // "A B"
	version   int    // a counter: the ETag
	checkETag bool   // false = last write wins
	stampMs   func() int64
	useStamp  bool // true = the ETag is a millisecond timestamp (the bug)
}

func (s *Store) etag() string {
	if s.useStamp {
		return `"t` + strconv.FormatInt(s.stampMs(), 10) + `"`
	}
	return `"v` + strconv.Itoa(s.version) + `"`
}

func (s *Store) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch r.Method {
	case http.MethodGet:
		tag := s.etag()
		if r.Header.Get("If-None-Match") == tag {
			w.WriteHeader(http.StatusNotModified) // your copy is still good
			return
		}
		w.Header().Set("ETag", tag)
		io.WriteString(w, s.addrs)
	case http.MethodPut:
		if s.checkETag && r.Header.Get("If-Match") != s.etag() {
			w.WriteHeader(http.StatusPreconditionFailed) // stale copy: refuse
			return
		}
		body, _ := io.ReadAll(r.Body)
		s.addrs = string(body)
		s.version++ // a counter never repeats
		w.Header().Set("ETag", s.etag())
		w.WriteHeader(http.StatusOK)
	}
}

func get(url string, h map[string]string) (int, string, string) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	for k, v := range h {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b), res.Header.Get("ETag")
}

func put(url, body, ifMatch string) (int, string) {
	req, _ := http.NewRequest(http.MethodPut, url, strings.NewReader(body))
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	res.Body.Close()
	return res.StatusCode, res.Header.Get("ETag")
}

func main() {
	fmt.Println("--- tab 1: last write wins (no If-Match check)")
	s := &Store{addrs: "A B", version: 5}
	srv := httptest.NewServer(s)
	u := srv.URL + "/watchlists/7"
	_, one, _ := get(u, nil)
	_, two, _ := get(u, nil)
	fmt.Println("both read:", one, "|", two)
	c, _ := put(u, one+" C", "")
	fmt.Println("agent one PUT A B C ->", c, "stored:", s.addrs)
	c, _ = put(u, two+" D", "")
	fmt.Println("agent two PUT A B D ->", c, "stored:", s.addrs, "(C is lost)")
	srv.Close()

	fmt.Println("--- tab 2: If-Match with an ETag")
	s = &Store{addrs: "A B", version: 5, checkETag: true}
	srv = httptest.NewServer(s)
	u = srv.URL + "/watchlists/7"
	_, one, tagOne := get(u, nil)
	_, two, tagTwo := get(u, nil)
	fmt.Println("both read:", one, tagOne, "|", two, tagTwo)
	c, tagOne = put(u, one+" C", tagOne)
	fmt.Println("agent one PUT If-Match v5 ->", c, "new ETag", tagOne, "stored:", s.addrs)
	c, _ = put(u, two+" D", tagTwo)
	fmt.Println("agent two PUT If-Match v5 ->", c, "stored:", s.addrs)
	_, two, tagTwo = get(u, nil)
	fmt.Println("agent two re-reads:", two, tagTwo)
	c, tagTwo = put(u, two+" D", tagTwo)
	fmt.Println("agent two PUT If-Match v6 ->", c, "new ETag", tagTwo, "stored:", s.addrs)
	c, _, _ = get(u, map[string]string{"If-None-Match": tagTwo})
	fmt.Println("agent one GET If-None-Match v7 ->", c)
	srv.Close()

	fmt.Println("--- failure: a timestamp as the ETag")
	s = &Store{addrs: "A B", checkETag: true, useStamp: true, stampMs: func() int64 { return 1712 }}
	srv = httptest.NewServer(s)
	u = srv.URL + "/watchlists/7"
	_, one, tagOne = get(u, nil)
	_, two, tagTwo = get(u, nil)
	c1, _ := put(u, one+" C", tagOne)
	c2, _ := put(u, two+" D", tagTwo) // same millisecond: same stamp
	fmt.Println("both PUT with", tagOne, "->", c1, c2, "stored:", s.addrs, "(C is lost)")
	srv.Close()
}
