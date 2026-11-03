// OAuth 2.0 code flow with PKCE (Proof Key for Code Exchange), in memory.
// A toy auth server, a toy API, and the failures: a stolen code, a token
// with no expiry, and a reused refresh token. Tokens are signed with
// Ed25519 to stay short; 14-jwt-refresh-sessions shows real RS256 JWTs.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

var b64 = base64.RawURLEncoding

type claims struct {
	Sub   string `json:"sub"`
	Scope string `json:"scope"`
	Exp   int    `json:"exp"`
	ID    string `json:"jti"`
}

type grant struct {
	challenge, scope string
	used             bool
}

type refreshToken struct {
	family string
	used   bool
}

type authServer struct {
	pub     ed25519.PublicKey
	priv    ed25519.PrivateKey
	codes   map[string]*grant
	refresh map[string]*refreshToken
	revoked map[string]bool // refresh families
	next    int
}

func random() string { b := make([]byte, 32); rand.Read(b); return b64.EncodeToString(b) }

// challengeOf hashes the secret verifier. Only the hash goes through the browser.
func challengeOf(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return b64.EncodeToString(h[:])
}

// authorize runs after sign-in and approval: it stores the challenge with a one-time code.
func (a *authServer) authorize(challenge, scope string) string {
	a.next++
	code := fmt.Sprintf("code-%d", a.next)
	a.codes[code] = &grant{challenge: challenge, scope: scope}
	return code
}

func (a *authServer) sign(c claims) string {
	body, _ := json.Marshal(c)
	return b64.EncodeToString(body) + "." + b64.EncodeToString(ed25519.Sign(a.priv, body))
}

// exchange swaps code plus verifier for tokens. The code alone is refused.
func (a *authServer) exchange(code, verifier string, now int) (access, refresh string, err error) {
	g, ok := a.codes[code]
	if !ok || g.used {
		return "", "", fmt.Errorf("invalid_grant: code unknown or used")
	}
	if challengeOf(verifier) != g.challenge {
		return "", "", fmt.Errorf("invalid_grant: verifier does not match")
	}
	g.used = true
	access = a.sign(claims{"ana", g.scope, now + 900, random()[:6]})
	refresh = "R-" + random()[:6]
	a.refresh[refresh] = &refreshToken{family: code}
	return access, refresh, nil
}

// refreshTokens rotates: a second use of an old token revokes the family.
func (a *authServer) refreshTokens(r string, now int) (string, string, error) {
	t, ok := a.refresh[r]
	if !ok || a.revoked[t.family] {
		return "", "", fmt.Errorf("invalid_grant: sign in again")
	}
	if t.used {
		a.revoked[t.family] = true
		return "", "", fmt.Errorf("invalid_grant: reuse, family revoked")
	}
	t.used = true
	next := "R-" + random()[:6]
	a.refresh[next] = &refreshToken{family: t.family}
	return a.sign(claims{"ana", "graph:read", now + 900, random()[:6]}), next, nil
}

func readToken(pub ed25519.PublicKey, tok string) (claims, bool) {
	var c claims
	for i := range tok {
		if tok[i] == '.' {
			body, _ := b64.DecodeString(tok[:i])
			sig, _ := b64.DecodeString(tok[i+1:])
			if !ed25519.Verify(pub, body, sig) {
				return c, false
			}
			json.Unmarshal(body, &c)
			return c, true
		}
	}
	return c, false
}

var denyList = map[string]bool{}

// apiBroken checks the signature only, so a token with no expiry lives on.
func apiBroken(pub ed25519.PublicKey, tok string) int {
	if _, ok := readToken(pub, tok); !ok {
		return 401
	}
	return 200
}

// api checks signature, expiry, deny list and scope. It never asks the auth server.
func api(pub ed25519.PublicKey, tok, scope string, now int) int {
	c, ok := readToken(pub, tok)
	if !ok || now >= c.Exp || denyList[c.ID] {
		return 401
	}
	if c.Scope != scope {
		return 403
	}
	return 200
}

func main() {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	a := &authServer{pub, priv, map[string]*grant{}, map[string]*refreshToken{}, map[string]bool{}, 0}

	fmt.Println("Tab 1: the flow")
	verifier := random()
	code := a.authorize(challengeOf(verifier), "graph:read")
	fmt.Println("  one-time code:", code)
	_, _, err := a.exchange(code, "attacker-has-no-verifier", 0)
	fmt.Println("  stolen code, no verifier ->", err)
	access, r1, err := a.exchange(code, verifier, 0)
	fmt.Println("  code + verifier ->", err == nil, "tokens issued")
	_, _, err = a.exchange(code, verifier, 0)
	fmt.Println("  same code again ->", err)
	fmt.Println("  API graph:read at second 10  ->", api(pub, access, "graph:read", 10))
	fmt.Println("  API lanes:write at second 10 ->", api(pub, access, "lanes:write", 10))
	fmt.Println("  API at second 900            ->", api(pub, access, "graph:read", 900))
	access2, r2, _ := a.refreshTokens(r1, 900)
	fmt.Println("  refresh R1 -> new access works:", api(pub, access2, "graph:read", 910))

	fmt.Println("Tab 2: what goes wrong")
	leaked := a.sign(claims{"svc", "graph:read", 1 << 40, "leak-1"}) // effectively no expiry
	fmt.Println("  leaked no-expiry token, signature only, year later ->", apiBroken(pub, leaked))
	short := a.sign(claims{"svc", "graph:read", 900, "leak-2"})
	fmt.Println("  15-minute token, a year later ->", api(pub, short, "graph:read", 31_536_000))
	denyList["leak-2"] = true
	fmt.Println("  deny list, inside its 15 min  ->", api(pub, short, "graph:read", 60))
	_, _, err = a.refreshTokens(r1, 950)
	fmt.Println("  attacker reuses R1 ->", err)
	_, _, err = a.refreshTokens(r2, 960)
	fmt.Println("  real app sends R2  ->", err)
}
