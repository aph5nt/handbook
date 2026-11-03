// Dynamic client registration (RFC 7591): a new client registers itself and
// gets a client id. Open registration needs consent for every new client.
// Run: go run .
package main

import (
	"encoding/json"
	"fmt"
)

type Client struct {
	ID           string   `json:"client_id"`
	Name         string   `json:"client_name"`
	RedirectURIs []string `json:"redirect_uris"`
}

type AuthServer struct {
	clients      map[string]Client
	next         int
	skipConsent  bool // (bug) reuse consent across clients
	consentAsked []string
}

func (a *AuthServer) metadata() string {
	return `{"registration_endpoint":"https://id.example.com/register"}`
}

// register answers POST /register. Anyone may call it.
func (a *AuthServer) register(name string, redirects []string) Client {
	a.next++
	c := Client{fmt.Sprintf("c-%d", 50+a.next), name, redirects}
	a.clients[c.ID] = c
	return c
}

// authorize decides whether the user sees a consent screen for this client.
func (a *AuthServer) authorize(id string, approve func(string) bool) string {
	c := a.clients[id]
	if a.skipConsent && len(a.consentAsked) > 0 { // (bug) someone consented once
		return "code sent to " + c.RedirectURIs[0] + " (consent skipped)"
	}
	a.consentAsked = append(a.consentAsked, id)
	if !approve(fmt.Sprintf("allow %q, redirect %s?", c.Name, c.RedirectURIs[0])) {
		return "denied, no code"
	}
	return "code sent to " + c.RedirectURIs[0]
}

func main() {
	a := &AuthServer{clients: map[string]Client{}}
	fmt.Println("0 metadata:", a.metadata())
	host := a.register("New host", []string{"http://localhost:3000/cb"})
	b, _ := json.Marshal(host)
	fmt.Println("2 registered:", string(b))
	yes := func(q string) bool { fmt.Println("  consent:", q, "-> approve"); return true }
	no := func(q string) bool { fmt.Println("  consent:", q, "-> deny"); return false }
	fmt.Println("3", a.authorize(host.ID, yes))

	bad := a.register("Helpful app", []string{"https://attacker.example/cb"})
	fmt.Println("4 attacker registered:", bad.ID)
	a.skipConsent = true
	fmt.Println("4 consent skipped:", a.authorize(bad.ID, no))
	a.skipConsent = false
	fmt.Println("4 fix, asked per client:", a.authorize(bad.ID, no))
}
