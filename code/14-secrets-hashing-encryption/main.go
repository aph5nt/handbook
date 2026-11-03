// Passwords are hashed, data is encrypted, secrets live in a vault.
// Tab 1: unsalted MD5 -> wrapped at once -> upgraded at login to a salted,
// slow hash. PBKDF2 (standard library) stands in for bcrypt and argon2id.
// Tab 2: a secret in git history, rotation, a run-time file, a push scan.
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

const iterations = 600_000 // tuned so one check costs real time

type row struct {
	scheme string // "md5", "wrapped" = slow(md5(pw)), "slow" = slow(pw)
	salt   []byte
	hash   []byte
}

func slow(secret, salt []byte) []byte {
	k, _ := pbkdf2.Key(sha256.New, string(secret), salt, iterations, 32)
	return k
}

func md5sum(pw string) []byte { h := md5.Sum([]byte(pw)); return h[:] }

// wrap upgrades every stored MD5 hash now, without knowing any password.
func wrap(r *row) {
	r.salt = make([]byte, 16)
	rand.Read(r.salt)
	r.hash, r.scheme = slow(r.hash, r.salt), "wrapped"
}

// login compares, then upgrades the row while it holds the plain password.
func login(r *row, pw string) bool {
	var got []byte
	switch r.scheme {
	case "wrapped":
		got = slow(md5sum(pw), r.salt)
	case "slow":
		got = slow([]byte(pw), r.salt)
	}
	if subtle.ConstantTimeCompare(got, r.hash) != 1 {
		return false
	}
	if r.scheme == "wrapped" {
		r.salt = make([]byte, 16)
		rand.Read(r.salt)
		r.hash, r.scheme = slow([]byte(pw), r.salt), "slow"
	}
	return true
}

// fastHash is the failure: one SHA-256, so a guess costs almost nothing.
func fastHash(pw string) [32]byte { return sha256.Sum256([]byte(pw)) }

// encryptInstead is the other failure: encryption is built to be undone.
func encryptInstead(key []byte, pw string) []byte {
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)
	return gcm.Seal(nonce, nonce, []byte(pw), nil)
}

// decryptStored is what anyone holding the one key can do to every row.
func decryptStored(key, stored []byte) string {
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	n := gcm.NonceSize()
	pw, _ := gcm.Open(nil, stored[:n], stored[n:], nil)
	return string(pw)
}

// guessesIn counts how many guesses one core makes in d.
func guessesIn(d time.Duration, hash func(string)) int {
	n, end := 0, time.Now().Add(d)
	for time.Now().Before(end) {
		hash(fmt.Sprint("guess", n))
		n++
	}
	return n
}

// ---------- Tab 2: secrets ----------

var secretPattern = regexp.MustCompile(`(?i)(password|token|secret)\s*[:=]\s*\S{6,}`)

// prePushScan refuses a commit that contains a secret pattern.
func prePushScan(diff string) error {
	if m := secretPattern.FindString(diff); m != "" {
		return fmt.Errorf("refused: secret found: %q", m)
	}
	return nil
}

func main() {
	fmt.Println("Tab 1: passwords")
	alice, bob := &row{scheme: "md5", hash: md5sum("summer2024")}, &row{scheme: "md5", hash: md5sum("summer2024")}
	fmt.Println("  unsalted MD5, same password, same hash:", hex.EncodeToString(alice.hash) == hex.EncodeToString(bob.hash))
	wrap(alice)
	wrap(bob)
	fmt.Println("  wrapped at once, hashes now differ:", hex.EncodeToString(alice.hash) != hex.EncodeToString(bob.hash))
	t := time.Now()
	fmt.Println("  alice logs in:", login(alice, "summer2024"), "scheme now:", alice.scheme)
	fmt.Println("  one slow check took about", time.Since(t).Round(10*time.Millisecond)/2)
	fmt.Println("  wrong password:", login(alice, "winter2024"))
	fast := guessesIn(100*time.Millisecond, func(s string) { fastHash(s) })
	slowN := guessesIn(100*time.Millisecond, func(s string) { slow([]byte(s), alice.salt) })
	fmt.Printf("  guesses per 100 ms on one core: SHA-256 %d, slow hash %d\n", fast, slowN)
	key := make([]byte, 32)
	rand.Read(key)
	fmt.Println("  encrypted instead, stolen key opens it:", decryptStored(key, encryptInstead(key, "summer2024")))
	fmt.Println("Tab 2: secrets")
	secretsDemo()

}

// secretsDemo walks tab 2: history keeps a deleted secret, rotation kills
// it, the pod reads a mounted file, and the push scan refuses the next one.
func secretsDemo() {
	history := []string{"config.yaml: password: hunter2db"}
	history = append(history, "revert: remove password")
	fmt.Println("  after the delete commit, history still holds:", history[0])
	dbPassword := "n3w-from-vault" // rotated at the database: hunter2db is now dead
	fmt.Println("  rotate: db password changed, hunter2db is now dead")
	dir, _ := os.MkdirTemp("", "vault-mount")
	defer os.RemoveAll(dir)
	os.WriteFile(filepath.Join(dir, "db-password"), []byte(dbPassword), 0o400)
	pw, _ := os.ReadFile(filepath.Join(dir, "db-password"))
	fmt.Println("  pod reads the mounted file at start:", len(pw), "bytes, never in git or the image")
	layers := []string{"FROM golang", "ARG GIT_TOKEN=ghp_example123", "RUN go build"}
	fmt.Println("  build argument kept in the image layers:", layers[1])
	fmt.Println("  push scan:", prePushScan("db:\n  password: hunter3db\n"))
	fmt.Println("  push scan, clean diff:", prePushScan("db:\n  password_file: /run/secrets/db\n"))
}
