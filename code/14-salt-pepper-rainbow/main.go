// Salt, pepper and rainbow tables. A precomputed table cracks unsalted
// hashes by lookup. A salt per user makes it useless. A pepper from the
// vault helps only until the server falls, so the hash must stay slow.
// PBKDF2 (standard library) stands in for argon2id.
package main

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

func sha(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

// buildTable is the attacker's precomputed table: hash to password. A real
// rainbow table compresses this; the lookup idea is the same.
func buildTable(words []string, salt string) map[string]string {
	t := map[string]string{}
	for _, w := range words {
		t[sha(salt+w)] = w
	}
	return t
}

type row struct{ user, salt, hash string }

// store hashes with a fresh random salt per user, mixes in the pepper, and
// uses the slow hash. The salt sits in the row; the pepper never does.
func store(user, pw string, pepper []byte) row {
	s := make([]byte, 16)
	rand.Read(s)
	return row{user, hex.EncodeToString(s), hex.EncodeToString(slowHash(pw, s, pepper))}
}

func slowHash(pw string, salt, pepper []byte) []byte {
	m := hmac.New(sha256.New, pepper)
	m.Write([]byte(pw))
	k, _ := pbkdf2.Key(sha256.New, string(m.Sum(nil)), salt, 600_000, 32)
	return k
}

// noSaltDemo shows equal hashes, one lookup, and why one shared salt fails.
func noSaltDemo() map[string]string {
	common := []string{"123456", "password", "summer2026", "qwerty"}
	alice, bob := sha("summer2026"), sha("summer2026")
	fmt.Println("  equal hashes:", alice == bob)
	table := buildTable(common, "")
	fmt.Println("  one lookup cracks both:", table[alice])
	fixed := "app-salt"
	a2, b2 := sha(fixed+"summer2026"), sha(fixed+"summer2026")
	fmt.Println("  one salt for all, still equal:", a2 == b2)
	fmt.Println("  one new table for that salt:", buildTable(common, fixed)[a2])
	return table
}

func main() {
	fmt.Println("Tab 1: no salt")
	table := noSaltDemo()

	fmt.Println("Tab 2: salt per user, pepper, slow")
	pepper := []byte("from-the-vault")
	ra, rb := store("alice", "summer2026", pepper), store("bob", "summer2026", pepper)
	fmt.Println("  hashes differ:", ra.hash != rb.hash, " salts stored:", ra.salt[:8]+"...", rb.salt[:8]+"...")
	_, found := table[ra.hash]
	fmt.Println("  rainbow table finds it:", found)
	salt, _ := hex.DecodeString(ra.salt)
	t := time.Now()
	slowHash("guess", salt, pepper)
	fmt.Println("  one guess, one user, on this machine: about", time.Since(t).Round(10*time.Millisecond))
	fast := hmac.New(sha256.New, pepper)
	t = time.Now()
	for i := 0; i < 100_000; i++ {
		fast.Reset()
		fast.Write([]byte("guess"))
		fast.Sum(nil)
	}
	fmt.Println("  trap: pepper leaked with a fast hash, 100,000 guesses take", time.Since(t).Round(time.Millisecond))
}
