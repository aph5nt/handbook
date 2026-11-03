// Remote MCP: OAuth 2.1 sign-in with an audience check, and sessions behind
// a load balancer. No network: handlers are called with httptest. Run: go run .
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
)

const self = "https://mcp.example.com"

var key = []byte("id.example.com signing key")

// A token is "aud.sig". The signature covers the audience.
func issue(aud string) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(aud))
	return aud + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func valid(tok string) (aud string, ok bool) {
	i := strings.LastIndex(tok, ".")
	if i < 0 {
		return "", false
	}
	aud = tok[:i]
	return aud, hmac.Equal([]byte(tok), []byte(issue(aud)))
}

func mcp(w http.ResponseWriter, r *http.Request) {
	tok, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !found {
		w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+self+`/.well-known/oauth-protected-resource"`)
		w.WriteHeader(401)
		return
	}
	aud, ok := valid(tok)
	if !ok || aud != self { // signature first, then the audience
		w.WriteHeader(401)
		fmt.Fprint(w, "wrong audience: ", aud)
		return
	}
	// The server calls the Company API with its OWN credential.
	fmt.Fprint(w, "200 result, Company API called with server-key, not the user token")
}

func post(tok string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/mcp", nil)
	if tok != "" {
		r.Header.Set("Authorization", "Bearer "+tok)
	}
	w := httptest.NewRecorder()
	mcp(w, r)
	return w
}

// A proxy with one shared client id at a third party must ask the user
// for consent per client that registered with the proxy itself.
func proxyAuthorize(client string, consented map[string]bool, check bool) string {
	if check && !consented[client] {
		return "consent screen: allow " + client + "?"
	}
	return "code sent to " + client + "'s redirect (third party skipped consent)"
}

// Sessions: each replica keeps its own map. The router picks a replica.
type Replica struct {
	name     string
	sessions map[string][]string // session id -> sent SSE events
}

func (r *Replica) handle(sid string) string {
	if _, ok := r.sessions[sid]; !ok {
		return r.name + ": 404 unknown session " + sid
	}
	return r.name + ": 200"
}

func main() {
	w := post("")
	fmt.Println("oauth 1:", w.Code, w.Header().Get("WWW-Authenticate"))
	fmt.Println("oauth 2: metadata -> authorization_servers [https://id.example.com]")
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	sum := sha256.Sum256([]byte(verifier))
	fmt.Println("oauth 3: code_challenge", base64.RawURLEncoding.EncodeToString(sum[:]), "resource", self)
	w = post(issue(self))
	fmt.Println("oauth 4-5:", w.Code, w.Body.String())
	w = post(issue("https://other.example.com"))
	fmt.Println("oauth 6:", w.Code, w.Body.String())

	consented := map[string]bool{"claude-desktop": true}
	fmt.Println("oauth 7 bug:", proxyAuthorize("attacker-app", consented, false))
	fmt.Println("oauth 7 fix:", proxyAuthorize("attacker-app", consented, true))

	r1 := &Replica{"replica 1", map[string][]string{}}
	r2 := &Replica{"replica 2", map[string][]string{}}
	r1.sessions["s-7f3"] = nil // initialize landed on replica 1: Mcp-Session-Id: s-7f3
	roundRobin := []*Replica{r1, r2}
	fmt.Println("session 3 round robin:", roundRobin[1].handle("s-7f3"))
	sticky := map[string]*Replica{"s-7f3": r1} // route by the Mcp-Session-Id header
	fmt.Println("session 4 sticky:", sticky["s-7f3"].handle("s-7f3"))

	for i := 1; i <= 45; i++ {
		r1.sessions["s-7f3"] = append(r1.sessions["s-7f3"], fmt.Sprint(i))
	}
	last := 41 // the stream dropped. Reconnect with Last-Event-ID: 41
	fmt.Println("session 5 resume after", last, "->", r1.sessions["s-7f3"][last:])
}
