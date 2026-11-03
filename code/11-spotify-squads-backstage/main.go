// Command spotify replays the Spotify lesson's two runnable parts: a Backstage
// style catalog with an owner check and a search before building, and one pipe
// into one table. The squad model is a team practice and has no code.
package main

import (
	"fmt"
	"strings"
)

type Catalog map[string]string // service -> owning squad

func (c Catalog) Register(service, owner string) error {
	if owner == "" {
		return fmt.Errorf("%s: no owner, refused", service)
	}
	c[service] = owner
	return nil
}

func (c Catalog) Before(need, squad string) string {
	for s, owner := range c {
		if strings.Contains(s, need) {
			return "reuse " + s + ", ask " + owner
		}
	}
	c.Register(need+"-service", squad)
	return "build " + need + "-service"
}

type Event struct{ User, Song string }

type Pipe struct{ Table []Event }

func (p *Pipe) Play(user, song string) {
	e := Event{user, song}
	p.Table = append(p.Table, e)
}

// Count is what royalties and recommendations both run.
func Count(table []Event) int { return len(table) }

// PrivateFeed loses every nth event, as a team's own feed can.
func PrivateFeed(table []Event, loseEvery int) (out []Event) {
	for i, e := range table {
		if (i+1)%loseEvery != 0 {
			out = append(out, e)
		}
	}
	return out
}

func main() {
	c := Catalog{}
	fmt.Println("org step 4: register with no owner:", c.Register("old-thing", ""))
	fmt.Println("org step 5: Discover builds:", c.Before("recommend", "Discover"))
	fmt.Println("org step 5: Radio builds its own, no search:", "recommend-radio")
	fmt.Println("org step 6: Radio searches first:", c.Before("recommend", "Radio"))

	p := &Pipe{}
	for i := 0; i < 1000; i++ {
		p.Play("Ola", "a song")
	}
	fmt.Println("events step 1-3: plays in the one table:", Count(p.Table))
	fmt.Println("events step 4: royalties", Count(p.Table), "recommendations", Count(p.Table))
	fmt.Println("events step 6: team X's feed:", Count(PrivateFeed(p.Table, 50)), "against the table:", Count(p.Table))
}
