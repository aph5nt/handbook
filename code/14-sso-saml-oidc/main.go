// Single sign-on, in memory. A toy identity provider (IdP) signs a statement
// and the app checks signature, audience and expiry. Tab 2: a leaver, whose
// long app session outlives the disabled account until SCIM removes it.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
)

type statement struct {
	User string `json:"user"`
	Aud  string `json:"aud"`
	Exp  int    `json:"exp"`
}

type idp struct {
	pub      ed25519.PublicKey
	priv     ed25519.PrivateKey
	disabled map[string]bool
	sessions map[string]bool // who already signed in at the IdP
}

// issue signs a statement; no prompt with an IdP session, nothing if disabled.
func (i *idp) issue(user, aud string, now int) ([]byte, []byte, string) {
	if i.disabled[user] {
		return nil, nil, "account disabled"
	}
	how := "no prompt"
	if !i.sessions[user] {
		i.sessions[user], how = true, "signed in with passkey"
	}
	body, _ := json.Marshal(statement{user, aud, now + 300})
	return body, ed25519.Sign(i.priv, body), how
}

// accept is the app's three checks: signature, audience, expiry.
func accept(pub ed25519.PublicKey, body, sig []byte, me string, now int) (string, error) {
	if !ed25519.Verify(pub, body, sig) {
		return "", fmt.Errorf("bad signature")
	}
	var s statement
	json.Unmarshal(body, &s)
	if s.Aud != me {
		return "", fmt.Errorf("audience is %s, not %s", s.Aud, me)
	}
	if now >= s.Exp {
		return "", fmt.Errorf("expired")
	}
	return s.User, nil
}

// acceptCareless: signature wrapping. Checks the signed part, reads the name beside it.
func acceptCareless(pub ed25519.PublicKey, signed, sig []byte, unsignedUser string) (string, error) {
	if !ed25519.Verify(pub, signed, sig) {
		return "", fmt.Errorf("bad signature")
	}
	return unsignedUser, nil // read from the wrong place
}

// ---------- Tab 2: the leaver ----------

type app struct {
	name    string
	ttl     int
	session map[string]int // user -> session end
}

// open serves from the app's own session, and goes back to the IdP only
// when that session has ended.
func (a *app) open(i *idp, user string, now int) string {
	if end, ok := a.session[user]; ok && now < end {
		return "200, still in"
	}
	delete(a.session, user)
	if _, _, how := i.issue(user, a.name, now); how == "account disabled" {
		return "refused: account disabled"
	}
	a.session[user] = now + a.ttl
	return "200, new session"
}

// scimRemove is the IdP telling the app to end the user's sessions now.
func (a *app) scimRemove(user string) { delete(a.session, user) }

func main() {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	co := &idp{pub, priv, map[string]bool{}, map[string]bool{}}
	fmt.Println("Tab 1: sign in once")
	body, sig, how := co.issue("ana", "analytics", 0)
	fmt.Println("  analytics, IdP:", how, " statement:", string(body))
	u, err := accept(pub, body, sig, "analytics", 10)
	fmt.Println("  analytics accepts ->", u, err)
	_, err = accept(pub, body, sig, "wiki", 10)
	fmt.Println("  same statement at the wiki ->", err)
	_, err = accept(pub, body, sig, "analytics", 400)
	fmt.Println("  after 300 s ->", err)
	b2, s2, how := co.issue("ana", "wiki", 20)
	u, _ = accept(pub, b2, s2, "wiki", 20)
	fmt.Println("  wiki, IdP:", how, "->", u)
	u, _ = acceptCareless(pub, body, sig, "admin")
	fmt.Println("  careless parser, wrapped name ->", u)
	_, err = accept(pub, []byte(`{"user":"admin","aud":"analytics","exp":999}`), sig, "analytics", 10)
	fmt.Println("  proper check, changed body ->", err)

	fmt.Println("Tab 2: the leaver")
	const hour = 3600
	analytics := &app{"analytics", 30 * 24 * hour, map[string]int{}}
	wiki := &app{"wiki", 8 * hour, map[string]int{}}
	analytics.open(co, "ana", 0)
	wiki.open(co, "ana", 0)
	co.disabled["ana"] = true
	fmt.Println("  disabled at the IdP. day 7, analytics ->", analytics.open(co, "ana", 7*24*hour))
	fmt.Println("  hour 9, wiki ->", wiki.open(co, "ana", 9*hour))
	analytics.scimRemove("ana")
	fmt.Println("  SCIM remove, analytics ->", analytics.open(co, "ana", 7*24*hour+1))
}
