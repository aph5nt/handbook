// Token audience: a token is written for one server. A server checks the
// signature and the audience, and never forwards the token. Run: go run .
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

var signingKey = []byte("authorisation server key")

type Token struct{ Sub, Aud, Sig string }

func sign(sub, aud string) string {
	m := hmac.New(sha256.New, signingKey)
	m.Write([]byte(sub + "|" + aud))
	return hex.EncodeToString(m.Sum(nil))[:8]
}

// issue honours resource=... (RFC 8707): the audience is what the client named.
func issue(sub, resource string) Token { return Token{sub, resource, sign(sub, resource)} }

type Server struct {
	Name       string
	CheckAud   bool
	Credential Token
	Next       *Server
}

func (s *Server) Handle(t Token) string {
	if t.Sig != sign(t.Sub, t.Aud) {
		return "401 bad signature"
	}
	if s.CheckAud && t.Aud != s.Name {
		return fmt.Sprintf("401 wrong audience: aud=%s, this is %s", t.Aud, s.Name)
	}
	return fmt.Sprintf("200 from %s, acting for %s", s.Name, t.Sub)
}

// call forwards to the next service: with the user's token (the bug),
// or with this server's own credential (the fix).
func (s *Server) call(user Token, passThrough bool) string {
	if r := s.Handle(user); r[:3] != "200" {
		return r
	}
	if passThrough {
		return s.Next.Handle(user)
	}
	return s.Next.Handle(s.Credential)
}

func main() {
	lookupURL, billingURL := "https://lookup.example.com", "https://billing.example.com"
	t := issue("user-42", lookupURL)
	fmt.Printf("1 token: sub=%s aud=%s sig=%s\n", t.Sub, t.Aud, t.Sig)
	billing := &Server{Name: billingURL, CheckAud: true}
	lookup := &Server{Name: lookupURL, CheckAud: true, Next: billing, Credential: issue("lookup-service", billingURL)}
	fmt.Println("2 at lookup:", lookup.Handle(t))
	fmt.Println("3 replayed at billing:", billing.Handle(t))

	careless := &Server{Name: billingURL}
	lookup.Next = careless
	fmt.Println("4 pass-through to a careless billing:", lookup.call(t, true))
	lookup.Next = billing
	fmt.Println("5 own credential:", lookup.call(t, false))
}
