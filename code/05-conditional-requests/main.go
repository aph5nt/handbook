// Conditional requests over real HTTP. If-None-Match saves a download (304).
// If-Match refuses a lost update (412). Tags built from the file time differ
// per server, so the check fails; a tag from the content does not.
package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

const facts = "facts of 2026-10-06"

func contentTag(body string) string {
	sum := sha256.Sum256([]byte(body))
	return fmt.Sprintf("%x", sum[:4])
}

// factsServer answers GET with the given tag.
func factsServer(tag string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("ETag", `"`+tag+`"`)
			if r.Header.Get("If-None-Match") == `"`+tag+`"` {
				w.WriteHeader(http.StatusNotModified) // no body
				return
			}
			io.WriteString(w, facts)
		}))
}

func get(url, inm string) (status, n int, etag string) {
	req, _ := http.NewRequest("GET", url, nil)
	if inm != "" {
		req.Header.Set("If-None-Match", inm)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, len(b), resp.Header.Get("ETag")
}

// Label is one row guarded by a version.
type Label struct {
	mu      sync.Mutex
	text    string
	version int
}

func (l *Label) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l.mu.Lock()
	defer l.mu.Unlock()
	tag := fmt.Sprintf(`"v%d"`, l.version)
	if r.Method == http.MethodPut {
		if m := r.Header.Get("If-Match"); m != "" && m != tag {
			w.WriteHeader(http.StatusPreconditionFailed) // 412: writes nothing
			return
		}
		b, _ := io.ReadAll(r.Body)
		l.text, l.version = string(b), l.version+1
		tag = fmt.Sprintf(`"v%d"`, l.version)
	}
	w.Header().Set("ETag", tag)
	io.WriteString(w, l.text)
}

func put(url, body, ifMatch string) int {
	req, _ := http.NewRequest(http.MethodPut, url, strings.NewReader(body))
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

// notModifiedSince is the older check. Last-Modified holds whole seconds only.
func notModifiedSince(mod, since time.Time) bool {
	unchanged := !mod.Truncate(time.Second).After(since) // older form: Last-Modified, only to the second
	return unchanged
}

func main() {
	fmt.Println("== reads: one server ==")
	s1 := factsServer("v7")
	st, n, tag := get(s1.URL, "")
	fmt.Printf("  first GET: %d, %d bytes, ETag %s\n", st, n, tag)
	st, n, _ = get(s1.URL, tag)
	fmt.Printf("  If-None-Match %s: %d, %d bytes\n", tag, st, n)
	s1.Close()

	fmt.Println("== reads: two servers, tag from the file time ==")
	a, b := factsServer("t-1001"), factsServer("t-2002")
	_, _, tag = get(a.URL, "")
	st, n, _ = get(b.URL, tag)
	fmt.Printf("  server 1 said %s, the check lands on server 2: %d, %d bytes sent again\n", tag, st, n)
	a.Close()
	b.Close()

	fmt.Println("== reads: two servers, tag from the content ==")
	a, b = factsServer(contentTag(facts)), factsServer(contentTag(facts))
	_, _, tag = get(a.URL, "")
	st, n, _ = get(b.URL, tag)
	fmt.Printf("  server 1 said %s, the check lands on server 2: %d, %d bytes\n", tag, st, n)
	fmt.Println("== the older form: Last-Modified, whole seconds ==")
	saved := time.Date(2026, 10, 6, 14, 0, 0, 0, time.UTC) // the Last-Modified the browser saved
	edited := saved.Add(400 * time.Millisecond)            // a second change in the same second
	fmt.Printf("  file changed 0.4 s later: answered 304=%v, the change is missed\n", notModifiedSince(edited, saved))
	a.Close()
	b.Close()

	for _, guarded := range []bool{false, true} {
		fmt.Printf("== writes, If-Match on Bob's save=%v ==\n", guarded)
		l := &Label{text: "Unknown", version: 7}
		srv := httptest.NewServer(l)
		code := put(srv.URL, "Mixer", `"v7"`)
		fmt.Printf("  Alice saves Mixer: %d, label now v%d\n", code, l.version)
		cond := ""
		if guarded {
			cond = `"v7"` // Bob read version 7
		}
		code = put(srv.URL, "Exchange", cond)
		fmt.Printf("  Bob saves Exchange: %d, label %q v%d\n", code, l.text, l.version)
		if guarded {
			code = put(srv.URL, "Exchange", `"v8"`)
			fmt.Printf("  Bob re-reads v8, saves with If-Match v8: %d, label %q v%d\n", code, l.text, l.version)
		}
		srv.Close()
	}
}
