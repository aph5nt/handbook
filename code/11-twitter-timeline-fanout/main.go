// Command twitter replays the timeline lesson: fan-out on write, fan-out on
// read, the delete clean-up, the celebrity problem and the hybrid fix.
// The star's 50 million followers are a count, not a list, so the run is fast.
package main

import (
	"fmt"
	"sort"
)

type Site struct {
	Store      map[int64]string
	Followers  map[string][]string // author -> follower names
	Count      map[string]int      // follower count when no list is kept
	Following  map[string][]string
	Timeline   map[string][]int64
	Recent     map[string][]int64 // each author's own tweets
	StarTweets map[string][]int64
	Hybrid     bool
	Writes     int
	next       int64
}

func (s *Site) Post(author, text string) int64 {
	s.next++
	id := s.next
	s.Store[id], s.Writes = text, s.Writes+1
	s.Recent[author] = append(s.Recent[author], id)
	if s.Hybrid && s.Count[author] > 10000 {
		s.StarTweets[author] = append(s.StarTweets[author], id)
		return id
	}
	for _, f := range s.Followers[author] {
		s.Timeline[f] = append(s.Timeline[f], id)
		s.Writes++
	}
	return id
}

func (s *Site) OpenWrite(u string) ([]int64, int) { return s.Timeline[u], 1 }

func (s *Site) OpenRead(u string) (ids []int64, lookups int) {
	for _, f := range s.Following[u] {
		ids, lookups = append(ids, s.Recent[f]...), lookups+1
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] > ids[j] })
	return ids, lookups
}

func (s *Site) OpenHybrid(u string, stars []string) ([]int64, int) {
	ids := append([]int64{}, s.Timeline[u]...)
	for _, st := range stars {
		ids = append(ids, s.StarTweets[st]...)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] > ids[j] })
	return ids, 1 + len(stars)
}

func (s *Site) Show(ids []int64) (texts []string) {
	for _, id := range ids {
		if t, ok := s.Store[id]; ok {
			texts = append(texts, t)
		}
	}
	return texts
}

func (s *Site) Delete(author string, id int64) (cleanups int) {
	delete(s.Store, id)
	for _, f := range s.Followers[author] {
		s.Timeline[f] = without(s.Timeline[f], id)
		cleanups++
	}
	return cleanups
}

func without(tl []int64, id int64) []int64 {
	for i, x := range tl {
		if x == id {
			return append(tl[:i], tl[i+1:]...)
		}
	}
	return tl
}

func newSite() *Site {
	return &Site{Store: map[int64]string{}, Followers: map[string][]string{}, Count: map[string]int{},
		Following: map[string][]string{}, Timeline: map[string][]int64{}, Recent: map[string][]int64{},
		StarTweets: map[string][]int64{}}
}

func main() {
	s := newSite()
	for i := 0; i < 299; i++ {
		s.Followers["ana"] = append(s.Followers["ana"], fmt.Sprintf("fan%d", i))
	}
	s.Followers["ana"] = append(s.Followers["ana"], "ben")
	fmt.Println("write tab")
	id := s.Post("ana", "hello")
	fmt.Printf("step 1-4: Ana posts id %d to %d followers: writes %d\n", id, len(s.Followers["ana"]), s.Writes)
	ids, n := s.OpenWrite("ben")
	fmt.Printf("step 5: Ben opens: %d lookup, %v\n", n, s.Show(ids))
	fmt.Printf("step 6: delete: %d timeline copies cleaned, Ben now sees %v\n", s.Delete("ana", id), s.Show(s.Timeline["ben"]))

	fmt.Println("read tab")
	r := newSite()
	r.Post("ana", "hello")
	fmt.Println("step 1: Ana posts: writes", r.Writes)
	for i := 0; i < 199; i++ {
		u := fmt.Sprintf("friend%d", i)
		r.Following["ben"] = append(r.Following["ben"], u)
		r.Post(u, "news")
	}
	r.Following["ben"] = append(r.Following["ben"], "ana")
	ids, n = r.OpenRead("ben")
	fmt.Printf("step 2-4: Ben follows %d: %d lookups, %d tweets merged, newest first id %d\n",
		len(r.Following["ben"]), n, len(ids), ids[0])

	fmt.Println("hybrid tab")
	h := newSite()
	h.Count["star"] = 50000000
	fmt.Printf("step 1-2: pure fan-out: star needs %d writes; at 300,000 per second that is %d s\n",
		h.Count["star"], h.Count["star"]/300000)
	h.Hybrid = true
	h.Followers["ana"] = s.Followers["ana"]
	h.Post("star", "new album")
	fmt.Println("step 3: hybrid: star posts, writes", h.Writes)
	h.Post("ana", "hello")
	fmt.Println("step 4: Ana posts, total writes", h.Writes)
	ids, n = h.OpenHybrid("ben", []string{"star"})
	fmt.Printf("step 5: Ben opens: %d lookups, %v\n", n, h.Show(ids))
}
