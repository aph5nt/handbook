// Command schemaless replays the Schemaless lesson: 4,096 fixed shards,
// append-only cells with versions, the newest-version read, a change stream
// that feeds billing and a driver index, and the index lag with its fix.
package main

import (
	"fmt"
	"hash/fnv"
)

type Cell struct {
	Row, Column string
	Version     int
	Body        string
}

type Store struct {
	Shards map[uint32][]Cell
	Stream []Cell
	Index  map[string][]string // driver -> rows
	Read   int                 // stream position of the index builder
}

func Shard(row string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(row))
	return h.Sum32() % 4096
}

func (s *Store) Put(row, column, body string) Cell {
	c := Cell{row, column, s.newest(row, column).Version + 1, body}
	s.Shards[Shard(row)] = append(s.Shards[Shard(row)], c)
	s.Stream = append(s.Stream, c)
	return c
}

func (s *Store) newest(row, column string) (best Cell) {
	for _, c := range s.Shards[Shard(row)] {
		if c.Row == row && c.Column == column && c.Version > best.Version {
			best = c
		}
	}
	return best
}

func Billing(c Cell) string {
	if c.Column == "STATUS" && c.Body == "finished" {
		return "charge rider of " + c.Row
	}
	return ""
}

func (s *Store) BuildIndex(driverOf func(Cell) string) {
	for ; s.Read < len(s.Stream); s.Read++ {
		c := s.Stream[s.Read]
		if c.Column == "BASE" {
			s.Index[driverOf(c)] = append(s.Index[driverOf(c)], c.Row)
		}
	}
}

func (s *Store) MyTrips(driver string, justWrote ...string) []string {
	out := append([]string{}, s.Index[driver]...)
	for _, row := range justWrote {
		if s.newest(row, "BASE").Version > 0 && !has(out, row) {
			out = append(out, row)
		}
	}
	return out
}

func has(l []string, x string) bool {
	for _, v := range l {
		if v == x {
			return true
		}
	}
	return false
}

func main() {
	s := &Store{Shards: map[uint32][]Cell{}, Index: map[string][]string{}}
	driver := func(Cell) string { return "ola" }
	fmt.Println("write tab")
	fmt.Printf("step 2: hash(trip 42) -> shard %d of 4096\n", Shard("trip 42"))
	s.Put("trip 42", "BASE", "rider Ana, driver Ola")
	c := s.Put("trip 42", "STATUS", "started")
	fmt.Printf("step 3: BASE v1 and STATUS v%d: %s\n", c.Version, c.Body)
	c = s.Put("trip 42", "STATUS", "finished")
	fmt.Printf("step 4: appended STATUS v%d: %s; shard now holds %d cells\n", c.Version, c.Body, len(s.Shards[Shard("trip 42")]))
	fmt.Printf("step 5: read STATUS -> v%d %s\n", s.newest("trip 42", "STATUS").Version, s.newest("trip 42", "STATUS").Body)
	fmt.Println("step 6: billing sees the stream ->", Billing(s.Stream[2]))

	fmt.Println("index tab")
	for i := 1; i <= 40; i++ {
		s.Put(fmt.Sprintf("trip %d", i), "BASE", "driver Ola")
	}
	s.BuildIndex(driver)
	fmt.Printf("step 1: Ola's list holds %d trips\n", len(s.MyTrips("ola")))
	s.Put("trip 43", "BASE", "rider Ben, driver Ola")
	fmt.Printf("step 2: trip 43 written to shard %d, read by key: v%d\n", Shard("trip 43"), s.newest("trip 43", "BASE").Version)
	fmt.Printf("step 3: Ola opens the list a second later: %d trips, has trip 43: %v\n", len(s.MyTrips("ola")), has(s.MyTrips("ola"), "trip 43"))
	s.BuildIndex(driver)
	fmt.Printf("step 4: builder catches up: %d trips\n", len(s.MyTrips("ola")))
	s.Put("trip 44", "BASE", "rider Eva, driver Ola")
	fmt.Printf("step 5: trip 44 written to shard %d; plain list %d trips, merged with my own write by key %d trips, has trip 44: %v\n",
		Shard("trip 44"), len(s.MyTrips("ola")), len(s.MyTrips("ola", "trip 44")), has(s.MyTrips("ola", "trip 44"), "trip 44"))
}
