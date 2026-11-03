// Crypto-shredding: one key per person, so erasing one key erases every
// copy of their data, even in an append-only log and in backups.
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
)

type keyStore struct {
	keys map[string][]byte
	log  []string
}

// keyFor returns this person's key, making it the first time.
func (s *keyStore) keyFor(person string) []byte {
	if s.keys[person] == nil {
		k := make([]byte, 32)
		rand.Read(k)
		s.keys[person] = k
	}
	return s.keys[person]
}

func encrypt(key []byte, plain string) []byte {
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)
	return gcm.Seal(nonce, nonce, []byte(plain), nil)
}

// read goes through the key store: it logs, and fails once the key is gone.
func (s *keyStore) read(analyst, person string, sealed []byte) (string, error) {
	k, ok := s.keys[person]
	if !ok {
		return "", fmt.Errorf("no such key for %s", person)
	}
	s.log = append(s.log, analyst+" read "+person)
	block, _ := aes.NewCipher(k)
	gcm, _ := cipher.NewGCM(block)
	n := gcm.NonceSize()
	plain, err := gcm.Open(nil, sealed[:n], sealed[n:], nil)
	return string(plain), err
}

// forget deletes one key. No row in any log or backup is touched.
func (s *keyStore) forget(person string) { delete(s.keys, person) }

func main() {
	s := &keyStore{keys: map[string][]byte{}}
	name := encrypt(s.keyFor("customer-42"), "Ada Lovelace")
	other := encrypt(s.keyFor("customer-43"), "Grace Hopper")
	eventLog, warehouse, backup := name, name, name // three copies, all encrypted
	fmt.Printf("  event log holds %x...\n", eventLog[:6])
	plain, _ := s.read("analyst-1", "customer-42", warehouse)
	fmt.Println("  analyst read ->", plain)
	keyBackup := &keyStore{keys: map[string][]byte{"customer-42": s.keyFor("customer-42")}}
	csv := "customer-42,Ada Lovelace" // the failure: a plain export
	s.forget("customer-42")
	_, err := s.read("analyst-1", "customer-42", warehouse)
	fmt.Println("  after forget, warehouse ->", err)
	_, err = s.read("analyst-1", "customer-42", backup)
	fmt.Println("  after forget, backup    ->", err)
	got, _ := s.read("analyst-1", "customer-43", other)
	fmt.Println("  the other customer      ->", got)
	fmt.Println("  key store log:", s.log)
	fmt.Println("  the plain CSV export is untouched:", csv)
	old, _ := keyBackup.read("restore", "customer-42", backup)
	fmt.Println("  the key store's own old backup still opens it:", old)
}
