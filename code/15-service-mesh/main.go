// What a sidecar does on each call: pick a version, retry with a backoff,
// respect a retry budget, measure. And what retries in two places cost.
package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"math/rand"
	"time"
)

var errTimeout = errors.New("timeout")

// Budget allows retries up to 20 percent of the calls.
type Budget struct{ Calls, Retries int }

func (b *Budget) allow() bool { return float64(b.Retries) < 0.2*float64(b.Calls) }

// Sidecar applies the mesh rule on every call.
type Sidecar struct {
	MaxTries int
	Budget   *Budget // nil: no budget
	Rand     *rand.Rand
	Sent     int           // calls that reached the service
	Waited   time.Duration // total backoff
	Cert     tls.Certificate
	MeshCA   *x509.CertPool
}

// pick is the traffic split: 90 percent to v1, 10 percent to v2.
func (s *Sidecar) pick() string {
	if s.Rand.Intn(100) < 90 {
		return "v1"
	}
	return "v2"
}

// tlsConfig is mutual TLS: show this service's certificate, demand the peer's.
func (s *Sidecar) tlsConfig() *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{s.Cert}, // its service identity
		ClientCAs:    s.MeshCA,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
}

// Call tries up to MaxTries, waiting 100 ms, then doubling, with jitter.
func (s *Sidecar) Call(send func() error) error {
	backoff := 100 * time.Millisecond
	var err error
	for try := 1; try <= s.MaxTries; try++ {
		if try > 1 {
			if s.Budget != nil && !s.Budget.allow() {
				return err // the budget is spent: fail fast
			}
			if s.Budget != nil {
				s.Budget.Retries++
			}
			jitter := time.Duration(s.Rand.Int63n(int64(backoff / 2)))
			s.Waited += backoff + jitter // a real sidecar sleeps here
			backoff *= 2
		}
		s.Sent++
		if s.Budget != nil && try == 1 {
			s.Budget.Calls++
		}
		if err = send(); err == nil {
			return nil
		}
	}
	return err
}

// appCall is the old retry loop in the API code: 3 tries of 3 tries.
func appCall(tries int, s *Sidecar, send func() error) error {
	var err error
	for i := 0; i < tries; i++ {
		if err = s.Call(send); err == nil {
			return nil
		}
	}
	return err
}

func main() {
	r := rand.New(rand.NewSource(1))
	down := func() error { return errTimeout }

	// the traffic split: 90 percent v1, 10 percent v2
	split := &Sidecar{Rand: r}
	got := map[string]int{}
	for i := 0; i < 10000; i++ {
		got[split.pick()]++
	}
	fmt.Printf("split of 10000 calls: v1 %d, v2 %d\n", got["v1"], got["v2"])
	fmt.Println("mutual TLS: the peer must show a certificate:", split.tlsConfig().ClientAuth == tls.RequireAndVerifyClientCert)

	one := &Sidecar{MaxTries: 3, Rand: r}
	one.Call(down)
	fmt.Printf("mesh only, service down: 1 request makes %d calls, backoff %v\n", one.Sent, one.Waited.Round(time.Millisecond))

	two := &Sidecar{MaxTries: 3, Rand: r}
	appCall(3, two, down)
	fmt.Printf("app and mesh, service down: 1 request makes %d calls\n", two.Sent)

	for _, b := range []*Budget{nil, {}} {
		s := &Sidecar{MaxTries: 3, Budget: b, Rand: r}
		for i := 0; i < 1000; i++ {
			s.Call(down)
		}
		name := "no budget"
		if b != nil {
			name = "20 percent budget"
		}
		fmt.Printf("%s, 1000 requests, service down: %d calls\n", name, s.Sent)
	}
}
