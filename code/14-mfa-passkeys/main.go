// MFA and passkeys, in memory, on names that do not exist. Tab 1: a TOTP
// code is valid wherever it is typed, so a fake page can pass it on.
// Tab 2: a passkey signs the site's name too, so the name decides.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// ---------- Tab 1: one-time code ----------

// totp is RFC 6238: an HMAC of the 30-second step, cut to six digits.
// Nothing in the input names the site the user is looking at.
func totp(secret []byte, unixTime int64) string {
	var step [8]byte
	binary.BigEndian.PutUint64(step[:], uint64(unixTime/30))
	m := hmac.New(sha1.New, secret)
	m.Write(step[:])
	sum := m.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	v := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", v%1_000_000)
}

type site struct {
	password string
	secret   []byte
}

// signIn is the real site: the password and the code of this 30-second step.
func (s site) signIn(password, code string, now int64) string {
	if password != s.password || code != totp(s.secret, now) {
		return "refused"
	}
	return "session cookie"
}

// pushApprove models push fatigue: the attacker keeps asking until the
// tired user taps approve. Nothing checks who started the sign-in.
func pushApprove(promptsUntilTap, sent int) bool { return sent >= promptsUntilTap }

// ---------- Tab 2: passkey ----------

// phone holds one private key per site name. The name is the lookup key.
type phone struct{ keys map[string]*ecdsa.PrivateKey }

func (p *phone) register(rpID string) *ecdsa.PublicKey {
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	p.keys[rpID] = k
	return &k.PublicKey
}

// sign is given the site name by the browser, never by the page.
func (p *phone) sign(browserAt string, challenge []byte) ([]byte, error) {
	k, ok := p.keys[browserAt]
	if !ok {
		return nil, fmt.Errorf("no key for %s", browserAt)
	}
	h := sha256.Sum256(append([]byte(browserAt), challenge...))
	return ecdsa.SignASN1(rand.Reader, k, h[:])
}

// verify is the real site: its own name, its own challenge, the stored key.
func verify(pub *ecdsa.PublicKey, rpID string, challenge, sig []byte) bool {
	h := sha256.Sum256(append([]byte(rpID), challenge...))
	return ecdsa.VerifyASN1(pub, h[:], sig)
}

func main() {
	fmt.Println("Tab 1: one-time code")
	real := site{password: "correct horse", secret: []byte("shared-at-enrolment")}
	now := int64(1_800_000_000)
	code := totp(real.secret, now)
	fmt.Println("  phone shows:", code)
	// The toy fake page just forwards what was typed to the toy site.
	typedPassword, typedCode := "correct horse", code
	fmt.Println("  passed on within the 30 s ->", real.signIn(typedPassword, typedCode, now+5))
	fmt.Println("  same code, 60 s later     ->", real.signIn(typedPassword, typedCode, now+60))
	fmt.Println("  push prompts until the tap: 12, sent 12 ->", pushApprove(12, 12))

	fmt.Println("Tab 2: passkey")
	ph := &phone{keys: map[string]*ecdsa.PrivateKey{}}
	pub := ph.register("login.example.com")
	challenge := make([]byte, 32)
	rand.Read(challenge)
	_, err := ph.sign("login.examp1e.com", challenge)
	fmt.Println("  browser at the fake name  ->", err)
	sig, _ := ph.sign("login.example.com", challenge)
	fmt.Println("  browser at the real name  -> verified:", verify(pub, "login.example.com", challenge, sig))
	other := make([]byte, 32)
	rand.Read(other)
	fmt.Println("  old signature, new challenge -> verified:", verify(pub, "login.example.com", other, sig))
}
