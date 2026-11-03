// Crypto-shredding: person 42's email is encrypted with a key that belongs
// to person 42 only. Four copies hold the ciphertext. Deleting the key makes
// all four unreadable. The trap tab deletes plain rows instead.
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

// KeyStore holds one key per person, kept apart from the data.
type KeyStore struct{ keys map[int][]byte }

func (k *KeyStore) KeyFor(person int) []byte {
	if key, ok := k.keys[person]; ok {
		return key
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}
	k.keys[person] = key
	return key
}

// Encrypt seals a personal field with the person's own key (AES-GCM).
func Encrypt(key, plain []byte) []byte {
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic(err)
	}
	return gcm.Seal(nonce, nonce, plain, nil)
}

// Decrypt opens a field. With the key gone, every copy is noise.
func (k *KeyStore) Decrypt(person int, sealed []byte) (string, error) {
	key, ok := k.keys[person]
	if !ok {
		return "", errors.New("key deleted: unreadable")
	}
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	n := gcm.NonceSize()
	plain, err := gcm.Open(nil, sealed[:n], sealed[n:], nil)
	return string(plain), err
}

// Shred erases a person everywhere: one delete in the key store.
func (k *KeyStore) Shred(person int) { delete(k.keys, person) }

var places = []string{"events table", "Iceberg snapshot from June", "backup from last month", "copy in the analytics lake"}

func show(ks *KeyStore, copies [][]byte) {
	for i, c := range copies {
		v, err := ks.Decrypt(42, c)
		if err != nil {
			v = err.Error()
		}
		fmt.Printf("  %-28s %s\n", places[i], v)
	}
}

func main() {
	ks := &KeyStore{keys: map[int][]byte{}}
	sealed := Encrypt(ks.KeyFor(42), []byte("anna@example.com"))
	copies := [][]byte{sealed, sealed, sealed, sealed} // the same ciphertext, copied
	keyBackup := map[int][]byte{42: ks.keys[42]}       // the trap: keys in the data backup
	fmt.Println("before erasure:")
	show(ks, copies)

	ks.Shred(42)
	fmt.Println("after DELETE key 42:")
	show(ks, copies)

	ks.keys = keyBackup // restore last month's backup, keys included
	fmt.Println("after restoring a backup that held the key store:")
	show(ks, copies)

	// trap tab: plain email, delete the live row only
	plain := map[string]string{}
	for _, p := range places {
		plain[p] = "anna@example.com"
	}
	delete(plain, "events table")
	left := 0
	for _, p := range places {
		if _, ok := plain[p]; ok {
			left++
		}
	}
	fmt.Printf("plain rows, DELETE on the live table: %d of 4 copies still readable\n", left) // prints 3
}
