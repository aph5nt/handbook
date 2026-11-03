// Machine identity. Tab 1: a hashed, scoped, expiring API key with two live
// keys for rotation. Tab 2: mutual TLS on 127.0.0.1 with a toy mesh CA:
// the warehouse checks the caller's certificate, then a policy by name.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/http/httptest"
	"time"
)

// ---------- Tab 1: API key ----------

type keyRecord struct {
	owner, scope string
	expires      int // day number; 0 means never (the old, bad kind)
	revoked      bool
}

var keyStore = map[string]*keyRecord{} // sha256(key) -> record; the key itself is never stored

func hash(key string) string { s := sha256.Sum256([]byte(key)); return hex.EncodeToString(s[:]) }

// createKey shows the key once and keeps only its hash.
func createKey(key, owner, scope string, expires int) string {
	keyStore[hash(key)] = &keyRecord{owner: owner, scope: scope, expires: expires}
	return key
}

// checkKey: find the record by hash, then expiry, revocation and scope.
func checkKey(key, action string, today int) int {
	rec, ok := keyStore[hash(key)]
	if !ok || rec.revoked || (rec.expires != 0 && today > rec.expires) {
		return http.StatusUnauthorized
	}
	if rec.scope != "*" && rec.scope != action {
		return http.StatusForbidden
	}
	return http.StatusOK
}

// ---------- Tab 2: mutual TLS ----------

var caKey, _ = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
var caCert *x509.Certificate

func newCA() {
	t := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "mesh CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, _ := x509.CreateCertificate(rand.Reader, t, t, &caKey.PublicKey, caKey)
	caCert, _ = x509.ParseCertificate(der)
}

// issue makes a workload certificate signed by the mesh CA.
func issue(name string, notAfter time.Time) tls.Certificate {
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	t := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: name},
		DNSNames: []string{name}, NotBefore: notAfter.Add(-48 * time.Hour), NotAfter: notAfter,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}}
	der, _ := x509.CreateCertificate(rand.Reader, t, caCert, &k.PublicKey, caKey)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: k}
}

// policy: which caller identities may call the warehouse.
var policy = map[string]bool{"pricing": true}

// warehouse demands a client certificate signed by the mesh CA, then
// applies the policy to the name inside it.
func warehouse(cert tls.Certificate, pool *x509.CertPool) *httptest.Server {
	s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caller := r.TLS.PeerCertificates[0].Subject.CommonName
		if !policy[caller] {
			http.Error(w, "denied by policy: "+caller, http.StatusForbidden)
			return
		}
		fmt.Fprint(w, "rows for "+caller)
	}))
	s.Config.ErrorLog = log.New(io.Discard, "", 0) // the refused handshake is the expected result
	// RequireAndVerifyClientCert: the caller must show a certificate signed by a
	// CA in ClientCAs and inside its NotBefore..NotAfter, or the handshake fails.
	s.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, ClientCAs: pool,
		ClientAuth: tls.RequireAndVerifyClientCert}
	s.StartTLS()
	return s
}

func call(url string, client tls.Certificate, pool *x509.CertPool) string {
	c := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
		Certificates: []tls.Certificate{client}, RootCAs: pool, ServerName: "warehouse"}}}
	resp, err := c.Get(url)
	if err != nil {
		return "handshake fails"
	}
	defer resp.Body.Close()
	b := make([]byte, 64)
	n, _ := resp.Body.Read(b)
	return fmt.Sprintf("%d %s", resp.StatusCode, b[:n])
}

func main() {
	fmt.Println("Tab 1: API key")
	keyA := createKey("ck_live_7f3a", "customer script", "read", 365)
	fmt.Println("  shown once:", keyA, " stored:", hash(keyA)[:12]+"...")
	fmt.Println("  read with key A   ->", checkKey(keyA, "read", 10))
	fmt.Println("  delete with key A ->", checkKey(keyA, "delete", 10))
	keyB := createKey("ck_live_91c2", "customer script", "read", 375)
	fmt.Println("  read with key B   ->", checkKey(keyB, "read", 11), "(key A still", checkKey(keyA, "read", 11), ")")
	keyStore[hash(keyA)].revoked = true
	fmt.Println("  key A retired     ->", checkKey(keyA, "read", 12))
	old := createKey("ck_old_0001", "another customer", "*", 0)
	fmt.Println("  old key, no scope, no expiry: delete ->", checkKey(old, "delete", 1200))
	keyStore[hash(old)].revoked = true
	fmt.Println("  old key revoked: delete ->", checkKey(old, "delete", 1200))

	fmt.Println("Tab 2: mutual TLS")
	newCA()
	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	later := time.Now().Add(12 * time.Hour)
	srv := warehouse(issue("warehouse", later), pool)
	defer srv.Close()
	fmt.Println("  pricing calls warehouse ->", call(srv.URL, issue("pricing", later), pool))
	fmt.Println("  web calls warehouse     ->", call(srv.URL, issue("web", later), pool))
	fmt.Println("  pricing, expired cert   ->", call(srv.URL, issue("pricing", time.Now().Add(-time.Minute)), pool))
	fmt.Println("  pricing, renewed cert   ->", call(srv.URL, issue("pricing", later), pool))
}
