// Images by digest: the digest is a hash of the bytes. A tag is a pointer
// that can move. Rebuilding one commit on a new base gives other bytes.
package main

import (
	"crypto/sha256"
	"fmt"
)

type Registry struct {
	blobs map[string][]byte // digest -> bytes
	tags  map[string]string // tag -> digest, can move
}

func digestOf(img []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(img)) }

func build(commit, base string) []byte { return []byte(base + "\n" + commit) }

func (r *Registry) push(img []byte, tag string) string {
	d := digestOf(img)
	r.blobs[d] = img
	r.tags[tag] = d // moves the tag if it already existed
	return d
}

// pullTag asks what the tag points at right now.
func (r *Registry) pullTag(tag string) string { return r.tags[tag] }

// pullDigest asks for exact bytes, and checks them.
func (r *Registry) pullDigest(d string) ([]byte, error) {
	img := r.blobs[d]
	if digestOf(img) != d {
		return nil, fmt.Errorf("digest mismatch for %s", d)
	}
	return img, nil
}

// promote copies the proven digest into production's values file.
func promote(staging, prod map[string]string) {
	prod["image"] = staging["image"] // a copy of a proven name, never a new build
}

// history is what any puller can read from the image's layers.
func history(buildArgs map[string]string, mountedSecrets []string) []string {
	var h []string
	for k, v := range buildArgs {
		h = append(h, "ARG "+k+"="+v) // the token ships with the image
	}
	for range mountedSecrets {
		h = append(h, "RUN --mount=type=secret (value not stored)")
	}
	return h
}

func short(d string) string { return d[7:11] }

func main() {
	reg := &Registry{map[string][]byte{}, map[string]string{}}
	ab := reg.push(build("a1b2c3", "golang base, Monday"), "v1.2.3")
	fmt.Println("Monday build of a1b2c3:", short(ab), " tag v1.2.3 ->", short(reg.pullTag("v1.2.3")))
	staging := reg.pullTag("v1.2.3")
	fmt.Println("staging pulls v1.2.3:", short(staging))

	cd := reg.push(build("a1b2c3", "golang base, Friday"), "v1.2.3")
	fmt.Println("Friday rebuild of a1b2c3:", short(cd), " tag v1.2.3 ->", short(reg.pullTag("v1.2.3")))
	fmt.Println("production pulls v1.2.3:", short(reg.pullTag("v1.2.3")), " same as what staging proved:", reg.pullTag("v1.2.3") == staging)
	fmt.Println("staging restarts, pulls v1.2.3:", short(reg.pullTag("v1.2.3")))

	if _, err := reg.pullDigest(ab); err == nil {
		fmt.Println("by digest, staging and production both get:", short(ab), " any day, after any restart")
	}
	stagingValues := map[string]string{"image": ab}
	prodValues := map[string]string{}
	promote(stagingValues, prodValues)
	if _, err := reg.pullDigest(prodValues["image"]); err == nil {
		fmt.Println("promoted by copy, production gets:", short(prodValues["image"]), " the bytes staging proved")
	}
	reg.blobs[ab] = []byte("tampered")
	_, err := reg.pullDigest(ab)
	fmt.Println("if the bytes change:", err != nil)

	fmt.Println("layer history with a build argument:", history(map[string]string{"GIT_TOKEN": "ghp_x1"}, nil))
	fmt.Println("layer history with a mounted secret:", history(nil, []string{"git_token"}))
}
