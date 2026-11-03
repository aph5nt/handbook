// Envelope encryption with a toy KMS (Key Management Service) in memory.
// The master key never leaves. Each file has its own data key, stored only
// in wrapped form. Rotating the master key re-wraps keys, not files.
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
)

func seal(key, plain []byte) []byte {
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)
	return gcm.Seal(nonce, nonce, plain, nil)
}

func open(key, sealed []byte) ([]byte, error) {
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	n := gcm.NonceSize()
	return gcm.Open(nil, sealed[:n], sealed[n:], nil)
}

type kms struct {
	master  []byte // the key-encryption key: no method ever returns it
	readers map[string]bool
	log     []string
}

// newDataKey returns the same fresh key twice: plain, and wrapped by master.
func (k *kms) newDataKey() (plain, wrapped []byte) {
	plain = make([]byte, 32)
	rand.Read(plain)
	return plain, seal(k.master, plain)
}

// unwrap checks the caller's permission and logs it before answering.
func (k *kms) unwrap(caller string, wrapped []byte) ([]byte, error) {
	if !k.readers[caller] {
		k.log = append(k.log, caller+" unwrap denied")
		return nil, fmt.Errorf("%s may not unwrap", caller)
	}
	k.log = append(k.log, caller+" unwrap ok")
	return open(k.master, wrapped)
}

// rotate makes a new master and re-wraps the data keys only.
func (k *kms) rotate(wrappedKeys [][]byte) [][]byte {
	next := make([]byte, 32)
	rand.Read(next)
	out := make([][]byte, len(wrappedKeys))
	for i, w := range wrappedKeys {
		dk, _ := open(k.master, w)
		out[i] = seal(next, dk)
	}
	k.master = next
	return out
}

type file struct{ body, wrappedKey []byte }

// encryptFile: a fresh data key, encrypt locally, forget the plain key.
func encryptFile(k *kms, data []byte) file {
	plain, wrapped := k.newDataKey()
	body := seal(plain, data)
	clear(plain)
	return file{body, wrapped}
}

func readFile(k *kms, caller string, f file) (string, error) {
	dk, err := k.unwrap(caller, f.wrappedKey)
	if err != nil {
		return "", err
	}
	b, err := open(dk, f.body)
	return string(b), err
}

func main() {
	master := make([]byte, 32)
	rand.Read(master)
	k := &kms{master: master, readers: map[string]bool{"report-job": true}}
	f := encryptFile(k, []byte("export of 2026-11-02"))
	fmt.Println("  stored:", len(f.body), "byte file +", len(f.wrappedKey), "byte wrapped key, no plain key")
	got, err := readFile(k, "report-job", f)
	fmt.Println("  report-job reads ->", got, err)
	_, err = readFile(k, "web", f)
	fmt.Println("  web reads ->", err)
	body := f.body
	f.wrappedKey = k.rotate([][]byte{f.wrappedKey})[0]
	got, _ = readFile(k, "report-job", file{body, f.wrappedKey})
	fmt.Println("  master rotated, file untouched ->", got)
	fmt.Println("  KMS log:", k.log)

	forever, _ := k.newDataKey() // the failure: cached forever and logged
	fmt.Printf("  debug log: dataKey=%x...\n", forever[:4])
	one, two := seal(forever, []byte("file one")), seal(forever, []byte("file two"))
	a, _ := open(forever, one)
	b, _ := open(forever, two)
	before := len(k.log)
	fmt.Println("  leaked key opens every file:", string(a), "+", string(b), " new KMS log lines:", len(k.log)-before)
}
