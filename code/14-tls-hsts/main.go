// TLS and HSTS on 127.0.0.1, with certificates made in this program.
// Tab 1: a client checks chain, name and date, and skip-verify accepts an
// impostor. Tab 2: Strict-Transport-Security makes a toy browser rewrite
// http to https before anything leaves.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
)

// selfCA makes an authority and one certificate for the name.
func selfCA(name string) (*x509.CertPool, tls.Certificate) {
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "toy root"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true,
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, _ := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	caCert, _ := x509.ParseCertificate(caDER)
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: name},
		DNSNames: []string{name}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, _ := x509.CreateCertificate(rand.Reader, leaf, caCert, &k.PublicKey, caKey)
	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	return pool, tls.Certificate{Certificate: [][]byte{der}, PrivateKey: k}
}

func server(cert tls.Certificate) *httptest.Server {
	s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		fmt.Fprint(w, "cases")
	}))
	s.Config.ErrorLog = log.New(io.Discard, "", 0) // refused handshakes are expected here
	s.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	s.StartTLS()
	return s
}

// clientGet: tls.Config checks the chain up to RootCAs, the name and the date.
func clientGet(url, name string, roots *x509.CertPool) string {
	c := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, ServerName: name}}}
	return fetch(c, url)
}

// clientSkipVerify is the failure: a test setting that shipped.
func clientSkipVerify(url string) string {
	c := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	return fetch(c, url)
}

func fetch(c *http.Client, url string) string {
	resp, err := c.Get(url)
	if err != nil {
		if i := strings.LastIndex(err.Error(), "tls:"); i >= 0 {
			return "refused: " + err.Error()[i:]
		}
		return "refused: " + err.Error()
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b) + ", " + tls.VersionName(resp.TLS.Version)
}

// ---------- Tab 2: HSTS ----------

type browser struct {
	hstsUntil map[string]int // host -> time the remembered rule ends
	preload   map[string]bool
}

// firstURL rewrites http to https before anything leaves, when it may.
func (b browser) firstURL(host string, now int) string {
	if b.preload[host] || now < b.hstsUntil[host] {
		return "https://" + host
	}
	return "http://" + host
}

// sawHeader stores the Strict-Transport-Security rule from a safe visit.
func (b browser) sawHeader(host string, maxAge, now int) { b.hstsUntil[host] = now + maxAge }

func main() {
	fmt.Println("Tab 1: TLS")
	roots, cert := selfCA("api.example.com")
	real := server(cert)
	defer real.Close()
	fmt.Println("  chain, name and date ok      ->", clientGet(real.URL, "api.example.com", roots))
	fmt.Println("  wrong name                   ->", clientGet(real.URL, "other.example.com", roots))
	_, impostor := selfCA("api.example.com")
	fake := server(impostor)
	defer fake.Close()
	fmt.Println("  impostor's certificate       ->", clientGet(fake.URL, "api.example.com", roots))
	fmt.Println("  impostor, skip verify on     ->", clientSkipVerify(fake.URL))

	fmt.Println("Tab 2: HSTS")
	b := browser{map[string]int{}, map[string]bool{}}
	host := "api.example.com"
	fmt.Println("  first visit, typed no https  ->", b.firstURL(host, 0), "(plain: strippable)")
	b.sawHeader(host, 31_536_000, 10)
	fmt.Println("  after the header, next visit ->", b.firstURL(host, 100))
	fmt.Println("  after a year and a day       ->", b.firstURL(host, 31_622_400))
	b.preload[host] = true
	fmt.Println("  on the preload list          ->", browser{map[string]int{}, b.preload}.firstURL(host, 0))
}
