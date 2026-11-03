// Rate limiting and load shedding: the story of the lesson
// 12-rate-limiting-and-load-shedding.html, replayed.
// A token bucket per API key stops a looping free-tier agent. A load shedder
// drops background work first when the whole graph is near its limit.
package main

import (
	"fmt"
	"math"
)

// Bucket is a token bucket: it holds up to Cap tokens, Rate tokens drip in per second.
type Bucket struct {
	Cap, Rate float64
	Tokens    float64
	Last      float64 // seconds
}

// Allow takes one token if there is one. Otherwise it returns the Retry-After wait.
func (b *Bucket) Allow(now float64) (bool, float64) {
	b.Tokens = math.Min(b.Cap, b.Tokens+(now-b.Last)*b.Rate) // drip in since last call
	b.Last = now
	if b.Tokens >= 1 {
		b.Tokens--
		return true, 0
	}
	return false, (1 - b.Tokens) / b.Rate // seconds until the next whole token
}

// Flood sends n requests spread evenly over secs seconds through the buckets,
// one after another (round robin), and counts served and refused.
func Flood(buckets []*Bucket, n int, secs float64) (served, refused int) {
	for k := 0; k < n; k++ {
		now := secs * float64(k) / float64(n)
		if ok, _ := buckets[k%len(buckets)].Allow(now); ok {
			served++
		} else {
			refused++
		}
	}
	return
}

func newBucket() *Bucket { return &Bucket{Cap: 10, Rate: 1, Tokens: 10} }

// Class is one caller class with its demand in percent of graph capacity.
type Class struct {
	Name     string
	Priority int // higher is kept longer
	Demand   float64
}

// Shed admits classes from the highest priority down while the total stays at or under limit.
func Shed(classes []Class, limit float64) (admitted float64, shed []string) {
	for _, c := range classes { // classes are sorted by priority, highest first
		if admitted+c.Demand <= limit {
			admitted += c.Demand
		} else {
			shed = append(shed, c.Name) // refused fast: 503, come back later
		}
	}
	return
}

func main() {
	fmt.Println("== Token bucket per key")
	b := newBucket()
	served := 0
	for k := 0; k < 10; k++ {
		if ok, _ := b.Allow(0); ok {
			served++
		}
	}
	fmt.Printf("step 2: burst of 10 at t=0: served %d, tokens left %.0f\n", served, b.Tokens)
	ok, wait := b.Allow(0)
	fmt.Printf("step 3: request 11 at t=0: allowed=%v, 429 Retry-After %.0f s\n", ok, wait)

	s, r := Flood([]*Bucket{newBucket()}, 1000, 60)
	fmt.Printf("step 4: 1,000 requests in 60 s, one bucket: served %d, refused %d\n", s, r)
	s, r = Flood([]*Bucket{newBucket(), newBucket(), newBucket()}, 1000, 60)
	fmt.Printf("step 5: same flood over 3 copies, 3 buckets: served %d, refused %d\n", s, r)
	s, r = Flood([]*Bucket{newBucket()}, 1000, 60)
	fmt.Printf("step 6: one shared bucket for all copies: served %d, refused %d\n", s, r)
	fmt.Println("step 7: fixed window of 60 per clock minute: 60 at 12:00:59 + 60 at 12:01:00 = 120 in 2 s")

	fmt.Println("== Load shedding at the peak")
	normal := []Class{{"paid", 3, 25}, {"free", 2, 22}, {"background", 1, 13}}
	a, sh := Shed(normal, 90)
	fmt.Printf("step 1: demand 60%% of capacity, limit 90%%: admitted %.0f%%, shed %v\n", a, sh)
	peak := []Class{{"paid", 3, 42}, {"free", 2, 38}, {"background", 1, 17}}
	fmt.Printf("step 2: peak demand %.0f%%, above the 90%% limit\n", 42.0+38+17)
	a, sh = Shed(peak, 90)
	fmt.Printf("step 3: shedder admits %.0f%%, sheds %v\n", a, sh)
	fmt.Printf("step 5: no shedder, spikes at 120 requests/s against 100/s capacity: queue grows %d per second, %d waiting after 30 s, wait %.0f s > 2 s timeout\n",
		120-100, (120-100)*30, float64((120-100)*30)/100)
}
