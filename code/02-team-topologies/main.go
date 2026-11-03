// Replays the team topologies lesson: three stream-aligned teams, a
// platform team, an enabling team and a complicated-subsystem team. Then it
// counts how long stream teams wait to deploy when the platform is a ticket
// queue, and when it is self-service.
package main

import "fmt"

type Team struct{ name, kind string }

type Link struct {
	from, to string
	mode     string // collaboration, X-as-a-service or facilitating
	weeks    int    // how long it lasts; 0 means for good
}

var teams = []Team{
	{"Ingest", "stream-aligned"}, {"Graph", "stream-aligned"}, {"Agent tools", "stream-aligned"},
	{"Platform", "platform"}, {"Enabling", "enabling"},
	{"Graph query engine", "complicated subsystem"},
}

var links = []Link{
	{"Platform", "Ingest", "X-as-a-service", 0},
	{"Platform", "Graph", "X-as-a-service", 0},
	{"Platform", "Agent tools", "X-as-a-service", 0},
	{"Enabling", "Ingest", "facilitating", 4},
	{"Graph query engine", "Agent tools", "X-as-a-service", 0},
}

const (
	days          = 10 // two working weeks
	deploysPerDay = 4  // per stream team
	ticketsPerDay = 8  // what the platform team can close in a day
)

type ticket struct {
	team string
	day  int
}

// deploy runs ten days of deploys. Self-service: the team runs the deploy
// itself, the same day. Tickets: every deploy waits in one queue for the
// platform team, which closes 8 a day, oldest first.
func deploy(selfService bool) (waits map[string]int, open []ticket, worst int) {
	waits = map[string]int{}
	var queue []ticket
	for d := 1; d <= days; d++ {
		for _, t := range teams {
			if t.kind != "stream-aligned" {
				continue
			}
			for i := 0; i < deploysPerDay; i++ {
				if selfService {
					continue // the team deploys now: wait 0 days
				}
				queue = append(queue, ticket{t.name, d}) // open a ticket
			}
		}
		for n := 0; n < ticketsPerDay && len(queue) > 0; n++ {
			tk := queue[0]
			queue = queue[1:]
			w := d - tk.day // days the deploy waited
			waits[tk.team] += w
			if w > worst {
				worst = w
			}
		}
	}
	return waits, queue, worst
}

func main() {
	stream := 0
	for _, t := range teams {
		if t.kind == "stream-aligned" {
			stream++
		}
	}
	fmt.Printf("teams: %d, stream-aligned: %d\n", len(teams), stream)
	for _, l := range links {
		last := "for good"
		if l.weeks > 0 {
			last = fmt.Sprintf("for %d weeks, then it moves on", l.weeks)
		}
		fmt.Printf("  %s -> %s: %s, %s\n", l.from, l.to, l.mode, last)
	}

	asked := days * deploysPerDay * stream
	for _, self := range []bool{true, false} {
		waits, open, worst := deploy(self)
		name := "platform as a ticket queue"
		if self {
			name = "platform as self-service"
		}
		total := 0
		for _, w := range waits {
			total += w
		}
		done := asked - len(open)
		fmt.Printf("%s: %d deploys asked in %d days, %d done, %d still waiting\n", name, asked, days, done, len(open))
		if self {
			fmt.Println("  every deploy waited 0 days")
			continue
		}
		fmt.Printf("  done deploys waited %.2f days on average, the worst %d days\n", float64(total)/float64(done), worst)
		fmt.Printf("  the oldest open ticket is %d days old on day %d\n", days-open[0].day, days)
		blocked := map[string]bool{}
		for _, tk := range open {
			blocked[tk.team] = true
		}
		fmt.Printf("  stream teams with a deploy still waiting: %d of %d\n", len(blocked), stream)
	}
}
