// Cache-aside on one story: the risk score of address 0x18. The app reads the
// cache, loads on a miss and sets with a TTL. Then the race: an update deletes
// the key before the database commits, and a reader puts the old value back.
// A fake clock and fixed interleavings keep the numbers exact on every run.
package main

import "fmt"

const ttl = 60 // seconds

var now = 0 // the fake clock, in seconds

type entry struct {
	val     int
	expires int
}

type Cache map[string]entry

func (c Cache) Get(key string) (int, bool) {
	e, ok := c[key]
	if !ok || now >= e.expires { // never set, or the TTL ran out
		return 0, false
	}
	return e.val, true
}

func (c Cache) Set(key string, v, ttl int) { c[key] = entry{v, now + ttl} }

func (c Cache) Delete(key string) { delete(c, key) }

// DB shows readers the last committed value only.
type DB struct {
	committed map[string]int
	queries   int
}

func (d *DB) Read(key string) int {
	d.queries++
	return d.committed[key]
}

func (d *DB) Commit(key string, v int) { d.committed[key] = v }

type Service struct {
	cache Cache
	db    *DB
}

func (s *Service) Read(key string) (int, string) {
	if v, ok := s.cache.Get(key); ok {
		return v, "hit"
	}
	v := s.db.Read(key)      // miss: the app reads the source
	s.cache.Set(key, v, ttl) // the app fills the cache
	return v, "miss, loaded and set"
}

// UpdateDeleteFirst is the bug: until the commit, readers still see the old
// value, so a reader can put it straight back.
func (s *Service) UpdateDeleteFirst(key string, v int, beforeCommit func()) {
	s.cache.Delete(key)
	beforeCommit() // the transaction is still open
	s.db.Commit(key, v)
}

// UpdateDeleteAfter is the fix: the delete runs after the commit, so the next
// miss can only load the new value.
func (s *Service) UpdateDeleteAfter(key string, v int, beforeCommit func()) {
	beforeCommit() // the transaction is still open
	s.db.Commit(key, v)
	s.cache.Delete(key)
}

func newService() *Service {
	return &Service{Cache{}, &DB{committed: map[string]int{"risk:0x18": 40}}}
}

func main() {
	const key = "risk:0x18"

	fmt.Println("== the flow: miss, load, set with a TTL ==")
	s := newService()
	v, how := s.Read(key)
	fmt.Printf("  t=%ds read: %d (%s), database queries %d\n", now, v, how, s.db.queries)
	now = 20
	v, how = s.Read(key)
	fmt.Printf("  t=%ds read: %d (%s), database queries %d\n", now, v, how, s.db.queries)
	now = 60
	v, how = s.Read(key)
	fmt.Printf("  t=%ds read: %d (%s), database queries %d\n", now, v, how, s.db.queries)

	fmt.Println("== the bug: delete the key, then commit ==")
	now = 0
	s = newService()
	s.Read(key) // the cache holds 40
	now = 10
	s.UpdateDeleteFirst(key, 90, func() {
		v, how := s.Read(key)
		fmt.Printf("  t=%ds during the update, a reader gets: %d (%s)\n", now, v, how)
	})
	now = 11
	v, how = s.Read(key)
	fmt.Printf("  t=%ds committed: database %d, reader gets %d (%s)\n", now, s.db.committed[key], v, how)
	now = 69
	v, how = s.Read(key)
	fmt.Printf("  t=%ds still wrong: reader gets %d (%s)\n", now, v, how)
	now = 70
	v, how = s.Read(key)
	fmt.Printf("  t=%ds the TTL ran out: reader gets %d (%s)\n", now, v, how)

	fmt.Println("== the fix: commit, then delete the key ==")
	now = 0
	s = newService()
	s.Read(key)
	now = 10
	s.UpdateDeleteAfter(key, 90, func() {
		v, how := s.Read(key)
		fmt.Printf("  t=%ds during the update, a reader gets: %d (%s), the old value is still the truth\n", now, v, how)
	})
	now = 11
	v, how = s.Read(key)
	fmt.Printf("  t=%ds committed and deleted: database %d, reader gets %d (%s)\n", now, s.db.committed[key], v, how)

	fmt.Println("== what the fix leaves: a slow reader ==")
	now = 0
	s = newService()
	now = 10
	old := s.db.Read(key) // a reader misses and reads 40, then stalls
	s.UpdateDeleteAfter(key, 90, func() {})
	s.cache.Set(key, old, ttl) // the stalled reader wakes and sets 40
	now = 11
	v, how = s.Read(key)
	fmt.Printf("  t=%ds database %d, reader gets %d (%s): rare, and the TTL ends it\n", now, s.db.committed[key], v, how)
	now = 70
	v, how = s.Read(key)
	fmt.Printf("  t=%ds the TTL ran out: reader gets %d (%s)\n", now, v, how)
}
