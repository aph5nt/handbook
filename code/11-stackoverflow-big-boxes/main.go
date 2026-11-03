// Command so replays the Stack Overflow lesson: reads stopped by cache layers,
// search sent elsewhere, then the two things a cache cannot fix: distance and
// writes. Machine sizes and rates are model figures, not measurements.
package main

import "fmt"

type Site struct {
	Output, Redis map[int]bool
	SQLReads      int
	Next          int
	Servers       int
}

func (s *Site) Read(q int) (server int, from string) {
	s.Next = s.Next%s.Servers + 1
	if s.Output[q] {
		return s.Next, "output cache"
	}
	if s.Redis[q] {
		s.Output[q] = true
		return s.Next, "Redis"
	}
	s.SQLReads++
	s.Redis[q], s.Output[q] = true, true
	return s.Next, "SQL Server"
}

func Search(words string) string { return "Elasticsearch" }

func (s *Site) Write(q int) string {
	delete(s.Output, q)
	delete(s.Redis, q)
	return "primary"
}

func Fits(writesPerSec, biggestBox int) string {
	if writesPerSec > biggestBox {
		return "no bigger box: shard or replicate"
	}
	return "fits"
}

func RoundTrip(serverMs, networkMs int) int { return serverMs + networkMs }

func Shape(readShare float64, regions int) string {
	if readShare >= 0.9 && regions == 1 {
		return "a few big boxes"
	}
	return "scale out"
}

func main() {
	s := &Site{Output: map[int]bool{}, Redis: map[int]bool{}, Servers: 9, Next: 1}
	s.Output[100], s.Redis[200] = true, true
	srv, from := s.Read(100)
	fmt.Printf("read step 2-3: web %d, popular question from %s, SQL reads %d\n", srv, from, s.SQLReads)
	srv, from = s.Read(200)
	fmt.Printf("read step 4: web %d, less popular from %s, SQL reads %d\n", srv, from, s.SQLReads)
	srv, from = s.Read(300)
	fmt.Printf("read step 5: web %d, old question from %s, SQL reads %d\n", srv, from, s.SQLReads)
	srv, from = s.Read(300)
	fmt.Printf("read step 5: the next reader, web %d, from %s, SQL reads %d\n", srv, from, s.SQLReads)
	fmt.Println("read step 6: search goes to", Search("nil pointer"))

	fmt.Println("breaks step 2: Sydney: 5 ms of server plus a 200 ms round trip:", RoundTrip(5, 200), "ms")
	fmt.Println("breaks step 3: a write goes to the", s.Write(100), "and clears the caches")
	fmt.Println("breaks step 4: writes 80 of 100:", Fits(80, 100), "; writes 150 of 100:", Fits(150, 100))
	fmt.Println("breaks step 5: 95% reads, 1 region:", Shape(0.95, 1), "; 50% reads, 3 regions:", Shape(0.5, 3))
}
