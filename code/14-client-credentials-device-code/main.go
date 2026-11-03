// Tokens with no browser redirect, on a toy auth server in memory.
// Client credentials: a service proves itself. Device code: a person
// approves on another device while the CLI polls.
package main

import (
	"fmt"
	"strings"
)

type client struct {
	secret  string
	scopes  []string
	revoked bool
}

type authServer struct {
	clients map[string]*client
	devices map[string]*deviceFlow // device code -> state
	clock   int
	nextID  int
}

type deviceFlow struct {
	userCode, app string
	approved      bool
	expires       int
}

// clientCredentials is the token endpoint: check the secret and the scope.
// There is no refresh token: the service simply asks again.
func (a *authServer) clientCredentials(id, secret, scope string) (string, error) {
	c, ok := a.clients[id]
	if !ok || c.revoked || c.secret != secret {
		return "", fmt.Errorf("invalid_client")
	}
	for _, s := range c.scopes {
		if s == scope {
			a.nextID++
			return fmt.Sprintf("tok-%d[%s, 15 min]", a.nextID, scope), nil
		}
	}
	return "", fmt.Errorf("invalid_scope: %s", scope)
}

// metricsAPI checks the Bearer token's scope against what this route needs.
func metricsAPI(bearer string) int {
	if !strings.Contains(bearer, "[metrics:read,") {
		return 403
	}
	return 200
}

// startDevice returns the device code for the CLI and a short user code.
func (a *authServer) startDevice(app, deviceCode, userCode string) (string, string) {
	a.devices[deviceCode] = &deviceFlow{userCode: userCode, app: app, expires: a.clock + 600}
	return deviceCode, userCode
}

// poll is what the CLI repeats every few seconds.
func (a *authServer) poll(deviceCode string) string {
	d, ok := a.devices[deviceCode]
	switch {
	case !ok || a.clock > d.expires:
		return "expired_token"
	case !d.approved:
		return "authorization_pending"
	}
	return "access + refresh token for " + d.app
}

// approve is the page the person opens. It shows the app's name, and the
// person answers whether they started this sign-in themselves.
func (a *authServer) approve(userCode string, startedByMe bool) string {
	for _, d := range a.devices {
		if d.userCode == userCode {
			if !startedByMe {
				return "not approved: page asked 'did you just start " + d.app + " here?'"
			}
			d.approved = true
			return "approved " + d.app
		}
	}
	return "unknown code"
}

func main() {
	a := &authServer{clients: map[string]*client{}, devices: map[string]*deviceFlow{}}
	fmt.Println("Tab 1: client credentials")
	a.clients["validator"] = &client{secret: "from-vault", scopes: []string{"metrics:read"}}
	tok, _ := a.clientCredentials("validator", "from-vault", "metrics:read")
	fmt.Println("  validator asks metrics:read ->", tok)
	fmt.Println("  metrics API with that token ->", metricsAPI(tok))
	_, err := a.clientCredentials("validator", "from-vault", "lanes:write")
	fmt.Println("  validator asks lanes:write  ->", err)
	a.clients["shared"] = &client{secret: "in-five-files", scopes: []string{"metrics:read", "lanes:write", "billing:read", "cases:read", "keys:admin"}}
	tok, _ = a.clientCredentials("shared", "in-five-files", "keys:admin")
	fmt.Println("  leaked shared secret, keys:admin ->", tok)
	a.clients["web"] = &client{secret: "web-key", scopes: []string{"cases:read"}}
	a.clients["validator"].revoked = true
	_, err = a.clientCredentials("validator", "from-vault", "metrics:read")
	fmt.Println("  one client revoked ->", err)
	tok, _ = a.clientCredentials("web", "web-key", "cases:read")
	fmt.Println("  the others go on   ->", tok)

	fmt.Println("Tab 2: device code")
	dc, uc := a.startDevice("cia CLI", "dev-81f", "WDJB-MJHT")
	fmt.Println("  CLI prints user code:", uc)
	fmt.Println("  poll ->", a.poll(dc))
	fmt.Println("  user:", a.approve(uc, true))
	fmt.Println("  poll ->", a.poll(dc))
	phished, pc := a.startDevice("attacker's tool", "dev-665", "KQPL-ZTRW")
	fmt.Println("  old page: victim enters the code:", a.approve(pc, true))
	fmt.Println("  attacker's poll ->", a.poll(phished))
	evil, code := a.startDevice("attacker's tool", "dev-666", "PMXV-GHRN")
	fmt.Println("  victim told 'enter this code', says no:", a.approve(code, false))
	fmt.Println("  attacker's poll ->", a.poll(evil))
	a.clock += 601
	fmt.Println("  after 10 minutes ->", a.poll(evil))
}
