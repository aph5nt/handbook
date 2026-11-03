// Supply chain and containers, in memory. Tab 1: a toy registry where a
// tag can move but a digest cannot, plus a signature check at admission.
// Tab 2: a NetworkPolicy that port-forward never meets, and a pod check.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
)

type registry struct {
	blobs map[string][]byte // digest -> image bytes; a digest names fixed bytes
	tags  map[string]string // tag -> digest; anyone with push rights can move it
}

func digest(b []byte) string { s := sha256.Sum256(b); return fmt.Sprintf("sha256:%x", s[:2]) }

func (r *registry) push(tag string, image []byte) string {
	d := digest(image)
	r.blobs[d], r.tags[tag] = image, d
	return d
}

// pull resolves "app:latest" through the tag table, "app@sha256:..."
// straight to the bytes, and re-hashes them so a digest cannot lie.
func (r *registry) pull(ref string) string {
	d := ref
	if t, ok := r.tags[ref]; ok {
		d = t
	}
	b := r.blobs[d]
	if digest(b) != d {
		return "refused: bytes do not match " + d
	}
	return string(b)
}

// admit is the admission check: the image digest must carry a signature
// from the release key, or the pod never starts.
func admit(pub ed25519.PublicKey, d string, sig []byte) bool {
	return ed25519.Verify(pub, []byte(d), sig)
}

// ---------- Tab 2 ----------

type netPolicy struct {
	enforced bool
	allowed  map[string]bool // callers listed for the API pod
}

// reach decides one call. port-forward enters through the kubelet on the
// node, so pod-to-pod policy never sees it.
func reach(p netPolicy, caller string, viaPortForward bool) string {
	if viaPortForward || !p.enforced || p.allowed[caller] {
		return "200"
	}
	return "dropped"
}

type podSpec struct {
	runAsNonRoot, readOnlyRoot, dropAllCaps, hasLimits bool
}

// hardened is what admission control demands of every pod.
func hardened(p podSpec) bool {
	return p.runAsNonRoot && p.readOnlyRoot && p.dropAllCaps && p.hasLimits
}

func main() {
	fmt.Println("Tab 1: pin by digest")
	reg := &registry{blobs: map[string][]byte{}, tags: map[string]string{}}
	old := reg.push("app:latest", []byte("build 41, last week"))
	tested := reg.push("app:latest", []byte("build 42, tested"))
	fmt.Println("  pull app:latest              ->", reg.pull("app:latest"))
	reg.push("app:latest", []byte("build 42-evil, pushed by a stolen account"))
	fmt.Println("  tag moved, pull app:latest   ->", reg.pull("app:latest"))
	fmt.Println("  pull app@"+tested+"     ->", reg.pull(tested))
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	sig := ed25519.Sign(priv, []byte(tested))
	fmt.Println("  admit tested digest, signed  ->", admit(pub, tested, sig))
	fmt.Println("  admit moved tag's digest     ->", admit(pub, reg.tags["app:latest"], sig))
	fmt.Println("  rollback app@"+old+"  ->", reg.pull(old))

	fmt.Println("Tab 2: NetworkPolicy and the pod")
	p := netPolicy{enforced: false, allowed: map[string]bool{"web": true}}
	fmt.Println("  laptop via port-forward      ->", reach(p, "laptop", true))
	p.enforced = true // after the cluster reboot
	for _, c := range []string{"validator", "ingress", "prometheus"} {
		fmt.Printf("  %-10s real call         -> %s\n", c, reach(p, c, false))
	}
	fmt.Println("  test from the validator pod  ->", reach(p, "validator", false), "(found before release)")
	p.allowed["validator"], p.allowed["ingress"], p.allowed["prometheus"] = true, true, true
	fmt.Println("  listed, validator again      ->", reach(p, "validator", false))
	fmt.Println("  pod as root, writable root   ->", hardened(podSpec{hasLimits: true}))
	fmt.Println("  non-root, read-only, no caps ->", hardened(podSpec{true, true, true, true}))
}
