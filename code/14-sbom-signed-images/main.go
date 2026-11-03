// SBOM (Software Bill of Materials) and signed images, in memory. CI makes
// image, SBOM and signature; admission checks the signature; a query over
// the SBOMs answers "who runs this library" in one pass.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
)

type image struct {
	name, digest string
	sbom         map[string]string // package -> version
	sig          []byte
}

type ci struct{ priv ed25519.PrivateKey }

// build makes all three: digest over the contents, the SBOM, the signature.
func (c ci) build(name string, packages map[string]string) image {
	h := sha256.New()
	fmt.Fprint(h, name)
	for _, k := range []string{"logging", "http", "json"} {
		fmt.Fprint(h, k, packages[k])
	}
	d := fmt.Sprintf("sha256:%x", h.Sum(nil)[:2])
	return image{name, d, packages, ed25519.Sign(c.priv, []byte(d))}
}

// admit is admission control: only a digest signed by this CI may run.
// With enforce off, as on the team that never checks, anything runs.
func admit(pub ed25519.PublicKey, img image, enforce bool) bool {
	if !enforce {
		return true
	}
	return len(img.sig) > 0 && ed25519.Verify(pub, []byte(img.digest), img.sig)
}

// affected asks every running image's SBOM for one vulnerable version.
func affected(running []image, pkg, version string) []string {
	var out []string
	for _, img := range running {
		if img.sbom[pkg] == version {
			out = append(out, img.name)
		}
	}
	return out
}

func main() {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	c := ci{priv}
	bad := "2.14.0"
	running := []image{
		c.build("api", map[string]string{"logging": bad, "http": "1.2"}),
		c.build("indexer", map[string]string{"logging": bad, "json": "3.1"}),
		c.build("web", map[string]string{"http": "1.2"}),
		c.build("worker", map[string]string{"logging": bad}),
	}
	fmt.Println("  admit api, signed by CI ->", admit(pub, running[0], true))
	hit := affected(running, "logging", bad)
	fmt.Println("  logging 2.14.0 is in   ->", hit)
	fixed := c.build("api", map[string]string{"logging": "2.17.1", "http": "1.2"})
	fmt.Println("  rebuilt api", fixed.digest, "admit ->", admit(pub, fixed, true))
	laptop := image{name: "api", digest: "sha256:beef", sbom: map[string]string{"logging": "2.17.1"}}
	fmt.Println("  laptop build, unsigned ->", admit(pub, laptop, true))
	fmt.Println("  team that never checks ->", admit(pub, laptop, false))
}
