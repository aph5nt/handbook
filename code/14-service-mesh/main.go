// A service mesh in memory: the control plane signs an identity for each
// workload, sidecars do mutual TLS and policy, and the app only speaks
// plain HTTP to its own sidecar. Strict mode refuses plain text.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
)

type identity struct {
	name string
	sig  []byte
}

type controlPlane struct {
	pub  ed25519.PublicKey
	priv ed25519.PrivateKey
}

// issue signs an identity for one workload's sidecar. A real mesh also sets a
// short expiry and uses X.509 certificates; 14-api-keys-mtls runs real mTLS.
func (c controlPlane) issue(name string) identity {
	return identity{name, ed25519.Sign(c.priv, []byte(name))}
}

type sidecar struct {
	me      identity
	trust   ed25519.PublicKey
	strict  bool
	policy  map[string]bool // "caller GET path" -> allowed
	metrics []string
}

// inbound is the receiving sidecar: identity from mTLS, or plain text from
// a pod with no sidecar, then the policy, then one metric line per call.
func (s *sidecar) inbound(peer *identity, path string, app func(string) string) string {
	caller := "plain text, no identity"
	switch {
	case peer == nil && s.strict:
		s.metrics = append(s.metrics, caller+" refused")
		return "refused: strict mode, mTLS only"
	case peer != nil:
		if !ed25519.Verify(s.trust, []byte(peer.name), peer.sig) {
			return "refused: not signed by the mesh CA"
		}
		caller = peer.name
		if !s.policy[caller+" GET "+path] {
			s.metrics = append(s.metrics, caller+" "+path+" 403")
			return "403: policy"
		}
	}
	s.metrics = append(s.metrics, caller+" "+path+" 200")
	return app(path)
}

// outbound is the calling sidecar: the app sends plain HTTP to localhost
// and the sidecar adds its identity on the way out.
func (s *sidecar) outbound(to *sidecar, path string, app func(string) string) string {
	return to.inbound(&s.me, path, app)
}

func main() {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	cp := controlPlane{pub, priv}
	warehouseApp := func(p string) string { return "rows for " + p }
	pricing := &sidecar{me: cp.issue("pricing"), trust: pub}
	web := &sidecar{me: cp.issue("web"), trust: pub}
	wh := &sidecar{me: cp.issue("warehouse"), trust: pub, policy: map[string]bool{"pricing GET /rows": true}}

	fmt.Println("  pricing app -> sidecar -> warehouse ->", pricing.outbound(wh, "/rows", warehouseApp))
	fmt.Println("  web -> warehouse                    ->", web.outbound(wh, "/rows", warehouseApp))
	fake := &identity{"pricing", []byte("not a real signature")}
	fmt.Println("  claims to be pricing, unsigned      ->", wh.inbound(fake, "/rows", warehouseApp))
	fmt.Println("  permissive, pod with no sidecar     ->", wh.inbound(nil, "/rows", warehouseApp))
	wh.strict = true
	fmt.Println("  strict, pod with no sidecar         ->", wh.inbound(nil, "/rows", warehouseApp))
	fmt.Println("  strict, pricing                     ->", pricing.outbound(wh, "/rows", warehouseApp))
	fmt.Println("  metrics at the warehouse sidecar:")
	for _, m := range wh.metrics {
		fmt.Println("   ", m)
	}
}
