// Signing keys. HS256: a shared secret, so one leak mints tokens for all.
// RS256 with a JSON Web Key Set (JWKS): public keys only at the API, a kid
// in each token, rotation in three steps, and the API picks the algorithm.
package main

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"fmt"
)

type token struct {
	alg, kid, claims string
	sig              []byte
}

func hs256(secret []byte, msg string) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(msg))
	return m.Sum(nil)
}

func rs256(k *rsa.PrivateKey, msg string) []byte {
	h := sha256.Sum256([]byte(msg))
	sig, _ := rsa.SignPKCS1v15(rand.Reader, k, crypto.SHA256, h[:])
	return sig
}

// verifyHS is the shared-secret API: anyone holding the secret can sign too.
func verifyHS(secret []byte, t token) bool {
	return hmac.Equal(hs256(secret, t.claims), t.sig)
}

type api struct {
	jwks      map[string]*rsa.PublicKey // cached keys by kid
	refetches int
	fetch     func() map[string]*rsa.PublicKey
}

// verifyCareless obeys the token's alg: an HS256 claim makes it use the
// public key's bytes as if they were a shared secret.
func (a *api) verifyCareless(t token) bool {
	pub := a.jwks[t.kid]
	if t.alg == "HS256" {
		der, _ := x509.MarshalPKIXPublicKey(pub)
		return hmac.Equal(hs256(der, t.claims), t.sig)
	}
	h := sha256.Sum256([]byte(t.claims))
	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, h[:], t.sig) == nil
}

// verify picks the algorithm itself: every key here is RS256 only. An
// unknown kid causes one rate-limited refetch of the JWKS.
func (a *api) verify(t token) (int, string) {
	pub, ok := a.jwks[t.kid]
	if !ok && a.refetches == 0 {
		a.refetches++
		a.jwks = a.fetch()
		pub, ok = a.jwks[t.kid]
	}
	if !ok {
		return 401, "unknown key " + t.kid
	}
	if t.alg != "RS256" {
		return 401, t.kid + " is RS256 only"
	}
	h := sha256.Sum256([]byte(t.claims))
	if rsa.VerifyPKCS1v15(pub, crypto.SHA256, h[:], t.sig) != nil {
		return 401, "bad signature"
	}
	return 200, "verified with " + t.kid
}

func main() {
	fmt.Println("Tab 1: HS256, then RS256")
	k := []byte("secret-K-copied-to-every-API")
	forged := token{alg: "HS256", claims: `{"role":"admin"}`}
	forged.sig = hs256(k, forged.claims) // signed with the leaked secret
	fmt.Println("  HS256, leaked K, forged admin token accepted ->", verifyHS(k, forged))

	k1, _ := rsa.GenerateKey(rand.Reader, 2048)
	k2, _ := rsa.GenerateKey(rand.Reader, 2048)
	attacker, _ := rsa.GenerateKey(rand.Reader, 2048)
	published := map[string]*rsa.PublicKey{"k1": &k1.PublicKey}
	fetch := func() map[string]*rsa.PublicKey { return published }
	a := &api{jwks: fetch(), fetch: fetch}
	good := token{"RS256", "k1", `{"sub":"ana"}`, rs256(k1, `{"sub":"ana"}`)}
	fmt.Print("  RS256, kid k1 -> ")
	fmt.Println(a.verify(good))
	fake := token{"RS256", "k1", `{"role":"admin"}`, rs256(attacker, `{"role":"admin"}`)}
	fmt.Print("  forged with another key -> ")
	fmt.Println(a.verify(fake))

	fmt.Println("Tab 2: rotate")
	published = map[string]*rsa.PublicKey{"k1": &k1.PublicKey, "k2": &k2.PublicKey}
	a.jwks = fetch()
	fmt.Println("  publish k2 first, still sign with k1:", len(a.jwks), "keys")
	newer := token{"RS256", "k2", `{"sub":"ana"}`, rs256(k2, `{"sub":"ana"}`)}
	fmt.Print("  token kid k2 -> ")
	fmt.Println(a.verify(newer))
	fmt.Print("  older token kid k1 -> ")
	fmt.Println(a.verify(good))
	published = map[string]*rsa.PublicKey{"k2": &k2.PublicKey}
	a.jwks = fetch()
	fmt.Print("  k1 retired -> ")
	fmt.Println(a.verify(good))
	unknown := token{"RS256", "k9", "{}", nil}
	fmt.Print("  kid k9, one refetch -> ")
	fmt.Println(a.verify(unknown))
	der, _ := x509.MarshalPKIXPublicKey(&k2.PublicKey)
	confused := token{alg: "HS256", kid: "k2", claims: `{"role":"admin"}`}
	confused.sig = hs256(der, confused.claims)
	fmt.Println("  careless, HS256 with the public key ->", a.verifyCareless(confused))
	fmt.Print("  fixed, same token -> ")
	fmt.Println(a.verify(confused))
}
