// Command slack replays the Slack lesson: shards by workspace, a router by
// channel, a fan-out read, a boot storm, and Flannel at the edge. Message and
// client counts are model figures.
package main

import "fmt"

func ByWorkspace(ws string) int {
	return map[string]int{"BigCorp": 1, "Studio": 2}[ws]
}

func ByChannel(channel string) int {
	switch c := channel[0]; {
	case c <= 'H':
		return 1
	case c <= 'P':
		return 2
	}
	return 3
}

// Reshard copies rows, then flips the router. Nothing is lost in the copy.
func Reshard(rows []string) map[int][]string {
	out := map[int][]string{}
	for _, ch := range rows {
		out[ByChannel(ch)] = append(out[ByChannel(ch)], ch)
	}
	return out
}

func FanOut(shards map[int][]string) (asked, rows int) {
	for _, r := range shards {
		asked++
		rows += len(r)
	}
	return
}

type Core struct{ Items int }

func (c *Core) Boot(everything int) { c.Items += everything }

type Flannel struct {
	Cache map[string]string
	Core  *Core
}

func (f *Flannel) Lookup(name string) string {
	if v, ok := f.Cache[name]; ok {
		return v
	}
	f.Core.Items++
	f.Cache[name] = "from core"
	return f.Cache[name]
}

func (f *Flannel) Change(name, v string) { f.Cache[name] = v }

func main() {
	msgs := map[int]int{}
	for ws, n := range map[string]int{"BigCorp": 900, "Studio": 60} {
		msgs[ByWorkspace(ws)] += n
	}
	msgs[3] = 40
	fmt.Println("shard step 2: messages per shard by workspace:", msgs[1], msgs[2], msgs[3])

	var rows []string
	for c := byte('A'); c <= 'Z'; c++ {
		for i := 0; i < 10; i++ {
			rows = append(rows, string(c)+fmt.Sprint(i))
		}
	}
	sh := Reshard(rows)
	fmt.Println("shard step 5: rows per shard by channel:", len(sh[1]), len(sh[2]), len(sh[3]))
	asked, n := FanOut(sh)
	fmt.Printf("shard step 6: unread across BigCorp: asked %d shards, merged %d rows\n", asked, n)

	c := &Core{}
	const clients, everything = 100000, 500
	for i := 0; i < clients; i++ {
		c.Boot(everything)
	}
	fmt.Println("edge step 3: boot storm items from the core:", c.Items)
	c2 := &Core{}
	f := &Flannel{Cache: map[string]string{"general": "12 members"}, Core: c2}
	fmt.Println("edge step 5: lookup general:", f.Lookup("general"), "; core items:", c2.Items)
	f.Lookup("rare")
	fmt.Println("edge step 5: lookup rare: core items:", c2.Items)
	f.Change("general", "renamed")
	fmt.Println("edge step 6:", f.Lookup("general"), "; core items:", c2.Items)
}
