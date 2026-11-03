// JWTs, refresh tokens and sessions on a toy issuer and API.
// Tab 1: a 15-minute RS256 access token is verified locally, expires, is
// refreshed with rotation, and a reused refresh token revokes the family.
// Tab 2: alg none, a 30-day expiry, no revocation, and the fixes.
package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type claims struct {
	Sub   string `json:"sub"`
	Scope string `json:"scope"`
	Exp   int64  `json:"exp"` // seconds on the toy clock
	Jti   string `json:"jti"` // token id, for the deny list
}

var key, _ = rsa.GenerateKey(rand.Reader, 2048)
var b64 = base64.RawURLEncoding
var denyList = map[string]bool{}

// sign builds header.claims.signature. alg "none" leaves the signature empty.
func sign(alg string, c claims) string {
	h, _ := json.Marshal(map[string]string{"alg": alg, "typ": "JWT"})
	p, _ := json.Marshal(c)
	body := b64.EncodeToString(h) + "." + b64.EncodeToString(p)
	if alg == "none" {
		return body + "."
	}
	sum := sha256.Sum256([]byte(body))
	sig, _ := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	return body + "." + b64.EncodeToString(sig)
}

func parse(tok string) (alg string, c claims, body string, sig []byte) {
	parts := strings.Split(tok, ".")
	var h map[string]string
	hb, _ := b64.DecodeString(parts[0])
	json.Unmarshal(hb, &h)
	pb, _ := b64.DecodeString(parts[1])
	json.Unmarshal(pb, &c)
	sig, _ = b64.DecodeString(parts[2])
	return h["alg"], c, parts[0] + "." + parts[1], sig
}

// verifyBroken lets the token's own header choose how it is checked.
func verifyBroken(tok string, now int64) error {
	alg, c, body, sig := parse(tok)
	if alg != "none" { // the token said "do not check me", and we obeyed
		sum := sha256.Sum256([]byte(body))
		if rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, sum[:], sig) != nil {
			return errors.New("401 bad signature")
		}
	}
	if now >= c.Exp {
		return errors.New("401 expired")
	}
	return nil
}

// verify pins RS256, checks the signature with the public key, the expiry
// and the deny list. It never calls the issuer.
func verify(tok string, now int64) error {
	alg, c, body, sig := parse(tok)
	if alg != "RS256" {
		return errors.New("401 only RS256")
	}
	sum := sha256.Sum256([]byte(body))
	if rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, sum[:], sig) != nil {
		return errors.New("401 bad signature")
	}
	if now >= c.Exp {
		return errors.New("401 expired")
	}
	if denyList[c.Jti] {
		return errors.New("401 on the deny list")
	}
	return nil
}

// The issuer keeps refresh tokens: which family, and whether used.
type issuer struct {
	family  map[string]string
	used    map[string]bool
	revoked map[string]bool
	n       int
}

func (s *issuer) issue(fam string, now int64) (access, refresh string) {
	s.n++
	access = sign("RS256", claims{Sub: "42", Scope: "graph:read", Exp: now + 15*60, Jti: fmt.Sprint("a", s.n)})
	refresh = fmt.Sprint("R", s.n)
	s.family[refresh] = fam
	return
}

// refresh rotates: one use per refresh token. A second use is theft.
func (s *issuer) refresh(r string, now int64) (string, string, error) {
	fam := s.family[r]
	if s.revoked[fam] {
		return "", "", errors.New("invalid_grant: family revoked, sign in again")
	}
	if s.used[r] {
		s.revoked[fam] = true // reuse: trust nobody in this family
		return "", "", errors.New("invalid_grant: " + r + " reused, family revoked")
	}
	s.used[r] = true
	a, nr := s.issue(fam, now)
	return a, nr, nil
}

// sessionValid is the browser-app check: a server-side row, deleted at once.
func sessionValid(sessions map[string]string, sid string) bool {
	_, ok := sessions[sid]
	return ok
}

func show(label string, err error) {
	if err == nil {
		fmt.Printf("  %-42s -> 200\n", label)
	} else {
		fmt.Printf("  %-42s -> %v\n", label, err)
	}
}

func main() {
	fmt.Println("Tab 1: a token's life")
	s := &issuer{family: map[string]string{}, used: map[string]bool{}, revoked: map[string]bool{}}
	access, r1 := s.issue("signin-1", 0)
	_, c, _, _ := parse(access)
	fmt.Printf("  sign in: claims sub=%s scope=%s exp=%ds, refresh %s\n", c.Sub, c.Scope, c.Exp, r1)
	show("t=0s API verifies locally", verify(access, 0))
	show("t=900s same token", verify(access, 900))
	a2, r2, err := s.refresh(r1, 900)
	fmt.Printf("  refresh %s -> new access, %s (err=%v)\n", r1, r2, err)
	show("t=900s new token", verify(a2, 900))
	_, _, err = s.refresh(r1, 1000)
	fmt.Println("  attacker reuses R1 ->", err)
	_, _, err = s.refresh(r2, 1000)
	fmt.Println("  real app refreshes R2 ->", err)

	fmt.Println("Tab 2: three classic mistakes")
	none := sign("none", claims{Sub: "x", Scope: "graph:read", Exp: 1 << 40})
	show("alg none, broken verifier", verifyBroken(none, 0))
	show("alg none, pinned verifier", verify(none, 0))
	leaked := sign("RS256", claims{Sub: "42", Exp: 30 * 86400, Jti: "leak"})
	show("30-day token, day 3", verify(leaked, 3*86400))
	short := sign("RS256", claims{Sub: "42", Exp: 15 * 60, Jti: "leak2"})
	denyList["leak2"] = true
	show("15-min token on the deny list, t=60s", verify(short, 60))
	sessions := map[string]string{"sid-9": "user 42"}
	delete(sessions, "sid-9")
	ok := sessionValid(sessions, "sid-9")
	fmt.Printf("  %-42s -> session found=%v, so 401 at once\n", "session cookie after row deleted", ok)
}
