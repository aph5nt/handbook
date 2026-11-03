// The confused deputy: a proxy with one client id at a provider sends a code
// to whichever client started the flow. The fix is per-client consent, an
// exact redirect match and a proxy-issued token for the client. Run: go run .
package main

import "fmt"

type Client struct{ ID, Redirect string }

type Provider struct{ consentCookie map[string]bool } // client id -> user consented

// authorize skips its screen when this browser consented to the client id before.
func (p *Provider) authorize(clientID string) (code string, skipped bool) {
	return "code-for-users-account", p.consentCookie[clientID]
}

type Proxy struct {
	clientID string // the one id this proxy holds at the provider
	clients  map[string]Client
	consent  map[string]bool // fix: per registered client
	fixed    bool
	ask      func(q string) bool
}

// register is dynamic client registration: anyone may call it.
func (x *Proxy) register(c Client) { x.clients[c.ID] = c }

// signIn runs one flow started by client id, with the redirect it asked for.
func (x *Proxy) signIn(id, redirect string, p *Provider) string {
	c, ok := x.clients[id]
	if !ok {
		return "400 unknown client"
	}
	if x.fixed {
		if c.Redirect != redirect { // character for character
			return "400 redirect does not match: " + redirect
		}
		if !x.consent[id] {
			if !x.ask("client " + c.Redirect + " wants access. Allow?") {
				return "denied: the user never reaches the provider"
			}
			x.consent[id] = true
		}
	}
	code, skipped := p.authorize(x.clientID)
	fmt.Printf("  provider: client id %s, consent screen skipped=%v\n", x.clientID, skipped)
	if x.fixed { // the provider's code stays inside: the client gets the proxy's own token
		return "proxy token (aud: proxy) for " + c.ID + " sent to " + c.Redirect
	}
	return code + " sent to " + c.Redirect
}

func main() {
	p := &Provider{consentCookie: map[string]bool{"proxy-shared-id": true}}
	x := &Proxy{clientID: "proxy-shared-id", clients: map[string]Client{}, consent: map[string]bool{},
		ask: func(q string) bool { fmt.Println("  proxy asks:", q, "-> deny"); return false }}
	x.register(Client{"attacker-app", "https://attacker.example/cb"})
	fmt.Println("attack:", x.signIn("attacker-app", "https://attacker.example/cb", p))

	x.fixed = true
	fmt.Println("fix:", x.signIn("attacker-app", "https://attacker.example/cb", p))
	x.register(Client{"claude-desktop", "http://localhost:3000/cb"})
	fmt.Println("fix, bad redirect:", x.signIn("claude-desktop", "http://localhost:3000/cb/../evil", p))
	x.consent["claude-desktop"] = true // the user allowed it on the proxy's own screen
	fmt.Println("fix, good client:", x.signIn("claude-desktop", "http://localhost:3000/cb", p))
}
