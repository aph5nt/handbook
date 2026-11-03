// Cache-aside, write-through and write-behind on one story: the price of a
// phone. A database, a cache, and what each pattern does on a write.
package main

import (
	"errors"
	"fmt"
)

type Store struct {
	db       int
	cache    int
	cached   bool
	dbWrites int
	dbFails  bool
	pending  int // write-behind: acknowledged writes not yet in the database
}

// ---- cache-aside: the app owns the logic ----

func (s *Store) AsideRead() (int, string) {
	if s.cached {
		return s.cache, "hit"
	}
	s.cache, s.cached = s.db, true // read the database, fill the cache
	return s.cache, "miss, filled"
}

func (s *Store) AsideWrite(v int, deleteKey bool) {
	s.db = v
	s.dbWrites++
	if deleteKey {
		s.cached = false // delete, do not update
	}
}

// ---- write-through: the write waits for both ----

func (s *Store) writeDB(v int) error {
	if s.dbFails {
		return errors.New("database write failed")
	}
	s.db = v
	s.dbWrites++
	return nil
}

// ThroughWriteBad stores in the cache first. A failed database write leaves
// a price the database never saved.
func (s *Store) ThroughWriteBad(v int) error {
	s.cache, s.cached = v, true
	return s.writeDB(v)
}

// ThroughWrite writes the database first, and on any error deletes the key.
func (s *Store) ThroughWrite(v int) error {
	if err := s.writeDB(v); err != nil {
		s.cached = false
		return err
	}
	s.cache, s.cached = v, true
	return nil
}

// ---- write-behind: acknowledged by the cache alone ----

func (s *Store) BehindWrite(v int) {
	s.cache, s.cached = v, true
	s.pending++
}

func (s *Store) Flush() {
	if s.pending > 0 {
		s.db = s.cache // many writes become one
		s.dbWrites++
		s.pending = 0
	}
}

func (s *Store) Crash() (lost int) {
	lost, s.pending, s.cached = s.pending, 0, false
	return lost
}

func main() {
	fmt.Println("== cache-aside ==")
	s := &Store{db: 500}
	v, how := s.AsideRead()
	fmt.Println("  first read:", v, how)
	v, how = s.AsideRead()
	fmt.Println("  second read:", v, how)
	s.AsideWrite(450, false)
	v, how = s.AsideRead()
	fmt.Printf("  write 450, no delete: read %d (%s), database %d: stale up to the 10 minute TTL\n", v, how, s.db)
	s.cached = false
	s.AsideRead()
	s.AsideWrite(450, true)
	v, how = s.AsideRead()
	fmt.Printf("  write 450, delete the key: read %d (%s)\n", v, how)

	fmt.Println("== write-through ==")
	s = &Store{db: 500, cache: 500, cached: true}
	s.ThroughWrite(450)
	v, how = s.AsideRead()
	fmt.Printf("  write 450: read %d (%s), database %d\n", v, how, s.db)
	s.dbFails = true
	err := s.ThroughWriteBad(400)
	fmt.Printf("  cache first, database fails (%v): cache %d, database %d\n", err, s.cache, s.db)
	s = &Store{db: 450, cache: 450, cached: true, dbFails: true}
	err = s.ThroughWrite(400)
	fmt.Printf("  database first, it fails (%v): key cached=%v, database %d\n", err, s.cached, s.db)

	fmt.Println("== write-behind ==")
	s = &Store{db: 500, cache: 500, cached: true}
	for _, p := range []int{450, 440, 430} {
		s.BehindWrite(p)
	}
	fmt.Printf("  3 writes acknowledged: cache %d, database %d\n", s.cache, s.db)
	s.Flush()
	fmt.Printf("  flush: database %d, database writes %d\n", s.db, s.dbWrites)
	s.BehindWrite(420)
	s.BehindWrite(410)
	lost := s.Crash()
	fmt.Printf("  420 and 410 acknowledged, then a crash: %d writes lost, database %d\n", lost, s.db)
}
