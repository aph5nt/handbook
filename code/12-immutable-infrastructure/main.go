// Command immutableinfrastructure replays the lesson. An image is named by the sha256
// of its content, so the same name always means the same bytes. A change is a new image.
// The other team edits servers by hand, and their fingerprints drift apart.
package main

import (
	"crypto/sha256"
	"fmt"
)

// Build makes one image from a commit. The digest is the hash of the content.
func Build(commit string, timeoutSec int) (digest, content string) {
	content = fmt.Sprintf("indexer 3.38, commit %s, timeout %d s", commit, timeoutSec)
	return Digest(content), content
}

// Digest is the first 4 hex digits of the sha256, enough for this story.
func Digest(content string) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(content)))[:11]
}

// Registry stores images by digest, and tags that can move.
type Registry struct {
	Images map[string]string // digest -> content, never changes
	Tags   map[string]string // tag -> digest, can be moved by any push
}

// Push stores the content under its digest and moves the tag.
func (r *Registry) Push(tag, digest, content string) {
	r.Images[digest] = content
	r.Tags[tag] = digest
}

// Pull fetches by digest and checks that the content hashes to that digest.
func (r *Registry) Pull(digest string) (string, bool) {
	content, found := r.Images[digest]
	return content, found && Digest(content) == digest
}

// Server is one machine: its packages and settings, as a fingerprint.
type Server struct {
	Name, TLSLib string
	TimeoutSec   int
}

// Fingerprint hashes what is on the box. Same setup, same fingerprint.
func (s Server) Fingerprint() string {
	return Digest(fmt.Sprintf("%s %d", s.TLSLib, s.TimeoutSec))
}

func main() {
	reg := &Registry{map[string]string{}, map[string]string{}}

	d1, c1 := Build("9e2a", 30)
	reg.Push("latest", d1, c1)
	fmt.Println("built from 9e2a:", d1)
	fmt.Println("staging tests pass on:", d1)
	for k := 1; k <= 3; k++ {
		_, ok := reg.Pull(d1) // the deploy pins the digest
		fmt.Printf("indexer %d runs %s, hash checked: %v\n", k, d1, ok)
	}

	d2, c2 := Build("c81f", 60)
	reg.Push("latest", d2, c2)
	fmt.Println("timeout 60 s, new commit c81f, new image:", d2)
	fmt.Println("rollback, deploy the old digest:", d1, "still means:", reg.Images[d1])

	// The other team: three servers set up by hand.
	s := []Server{{"server 1", "tls 1.0", 30}, {"server 2", "tls 1.0", 30}, {"server 3", "tls 1.0", 30}}
	fmt.Printf("\n2025: %s %s %s\n", s[0].Fingerprint(), s[1].Fingerprint(), s[2].Fingerprint())
	s[0].TLSLib = "tls 1.1" // May: a hand patch on server 1
	s[1].TimeoutSec = 60    // June: a timeout raised during an incident
	fmt.Printf("June: %s %s %s\n", s[0].Fingerprint(), s[1].Fingerprint(), s[2].Fingerprint())

	notes := Server{"server 2, rebuilt from the notes", "tls 1.0", 30}
	const neededSec = 45 // what real load takes
	fmt.Printf("server 2 dies, rebuilt from the notes: timeout %d s, load needs %d s: times out: %v\n",
		notes.TimeoutSec, neededSec, notes.TimeoutSec < neededSec)

	// The tag trap: same name, different code.
	monday := reg.Tags["latest"]
	d3, c3 := Build("d4a7", 60)
	reg.Push("latest", d3, c3)
	tuesday := reg.Tags["latest"]
	fmt.Printf("\ntag latest pulled Monday: %s, Tuesday: %s, same code: %v\n", monday, tuesday, monday == tuesday)
}
