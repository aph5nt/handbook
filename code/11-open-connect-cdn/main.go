// Command oc replays the Open Connect lesson: a night fill by predicted
// views, steering that ranks healthy boxes holding the file, streaming from
// inside the provider, a fill miss, a box that dies mid-film, and an AWS
// problem that does not touch the video.
package main

import (
	"fmt"
	"sort"
	"strings"
)

type Box struct {
	Name     string
	Hops     int
	Capacity int
	Titles   map[string]bool
	Healthy  bool
}

func Fill(b *Box, byViews []string) {
	for _, t := range byViews {
		if len(b.Titles) == b.Capacity {
			return
		}
		b.Titles[t] = true
	}
}

func Steer(title string, nearby []*Box) []*Box {
	var list []*Box
	for _, b := range nearby {
		if b.Titles[title] && b.Healthy {
			list = append(list, b)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Hops < list[j].Hops })
	return list
}

// Play streams from the list; dies says at which minute a box stalls.
func Play(list []*Box, minutes int, dies map[string]int) []string {
	var log []string
	at := 0
	for _, b := range list {
		stall, ok := dies[b.Name]
		if !ok || stall >= minutes {
			return append(log, fmt.Sprintf("%s min %d-%d, ended", b.Name, at, minutes))
		}
		log = append(log, fmt.Sprintf("%s min %d-%d, stalled", b.Name, at, stall))
		at = stall
	}
	return append(log, "no box left")
}

func names(l []*Box) string {
	var s []string
	for _, b := range l {
		s = append(s, fmt.Sprintf("%s (%d hops)", b.Name, b.Hops))
	}
	return strings.Join(s, ", ")
}

func main() {
	box1 := &Box{Name: "box 1", Hops: 2, Capacity: 3, Titles: map[string]bool{}, Healthy: true}
	box2 := &Box{Name: "box 2", Hops: 5, Capacity: 5, Titles: map[string]bool{}, Healthy: true}
	nearby := []*Box{box2, box1}
	predicted := []string{"The Crown 6", "Wednesday 1", "Dune", "Ozark 4", "Episode 5 (tomorrow)"}
	Fill(box1, predicted)
	Fill(box2, predicted)
	fmt.Printf("step 1: night fill: box 1 holds %d titles (full), box 2 holds %d\n", len(box1.Titles), len(box2.Titles))
	fmt.Println("step 2-3: steer(Dune) ->", names(Steer("Dune", nearby)))
	fmt.Println("step 4: play Dune, 120 min ->", Play(Steer("Dune", nearby), 120, nil))
	fmt.Println("step 5: steer(Episode 5 (tomorrow)) ->", names(Steer("Episode 5 (tomorrow)", nearby)))
	list := Steer("Dune", nearby)
	fmt.Println("step 6: box 1 disk fails at minute 40 ->", Play(list, 120, map[string]int{"box 1": 40}))
	fmt.Println("step 7: AWS has trouble after steering: the list is already on the TV ->", Play(list, 120, nil))
}
