// PKCE (Proof Key for Code Exchange): a stolen code is useless without the
// verifier, as long as the method is S256. Run: go run .
package main

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

func s256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

type AuthServer struct {
	challenge, method, code string
	used                    bool
}

// authorize stores the challenge and hands out a one-time code.
func (a *AuthServer) authorize(challenge, method string) string {
	a.challenge, a.method, a.used = challenge, method, false
	a.code = "code-7f3a"
	return a.code
}

// token swaps the code. The check is on the verifier, not on who is asking.
func (a *AuthServer) token(code, verifier string) string {
	if code != a.code {
		return "400 invalid_grant: unknown code"
	}
	if verifier == "" {
		return "400 invalid_grant: no code_verifier"
	}
	got := verifier
	if a.method == "S256" {
		got = s256(verifier)
	}
	if got != a.challenge {
		return "400 invalid_grant: hash(verifier) != challenge"
	}
	if a.used {
		return "400 invalid_grant: code already used"
	}
	a.used = true
	return "200 access_token"
}

func main() {
	// RFC 7636 appendix B test vector.
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	challenge := s256(verifier)
	fmt.Println("0 verifier", verifier)
	fmt.Println("0 authorize: code_challenge", challenge, "method S256")
	a := &AuthServer{}
	code := a.authorize(challenge, "S256")
	fmt.Println("1 redirect: code", code)
	fmt.Println("2 malicious app copies", code)
	fmt.Println("3 thief, no verifier:", a.token(code, ""))
	fmt.Println("3 thief, guessed verifier:", a.token(code, "guess"))
	fmt.Println("4 real client:", a.token(code, verifier))

	code = a.authorize(verifier, "plain") // the challenge IS the verifier
	seen := verifier                      // anyone who saw the first request has it
	fmt.Println("5 plain method, thief:", a.token(code, seen))
}
