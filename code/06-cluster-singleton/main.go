// Cluster singleton: the oldest node runs the one scheduler. Proxies buffer during a handover.
// A partition with no resolver makes two "oldest" nodes, and so two schedulers.
package main

import (
	"fmt"
	"sync"
)

type Book struct {
	Device int
	Slot   string
	Tech   string
	Reply  chan string
}

// Journal is shared storage: it survives the node.
type Journal struct {
	mu   sync.Mutex
	rows []Book
}

func (j *Journal) Append(b Book) { j.mu.Lock(); j.rows = append(j.rows, b); j.mu.Unlock() }
func (j *Journal) All() []Book {
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]Book(nil), j.rows...)
}

// startScheduler is what the manager on the oldest node does. It replays the journal first.
func startScheduler(node int, j *Journal) chan Book {
	mailbox := make(chan Book, 16)
	go func() {
		booked := map[string]string{}
		for _, r := range j.All() { // recover every booking the old one made
			booked[fmt.Sprintf("%d@%s", r.Device, r.Slot)] = r.Tech
		}
		fmt.Printf("  node %d: scheduler started, recovered %d booking(s)\n", node, len(booked))
		for b := range mailbox {
			key := fmt.Sprintf("%d@%s", b.Device, b.Slot)
			if who, taken := booked[key]; taken {
				b.Reply <- "refused, already " + who
				continue
			}
			booked[key] = b.Tech
			j.Append(b)
			b.Reply <- fmt.Sprintf("booked by node %d scheduler", node)
		}
	}()
	return mailbox
}

// Proxy forwards to wherever the singleton is, and buffers while there is none.
type Proxy struct {
	target chan Book
	buffer []Book
}

func (p *Proxy) Tell(b Book) {
	if p.target == nil {
		p.buffer = append(p.buffer, b)
		return
	}
	p.target <- b
}

func (p *Proxy) SetTarget(t chan Book) {
	p.target = t
	for _, b := range p.buffer {
		t <- b
	}
	fmt.Printf("  proxy: %d buffered booking(s) sent on\n", len(p.buffer))
	p.buffer = nil
}

func oldest(members []int) int { return members[0] } // members are kept oldest first

func book(p *Proxy, device int, slot, tech string) string {
	r := make(chan string, 1)
	p.Tell(Book{device, slot, tech, r})
	return <-r
}

func main() {
	fmt.Println("== normal, then handover")
	j := &Journal{}
	members := []int{1, 2, 3}
	s1 := startScheduler(oldest(members), j)
	p2 := &Proxy{target: s1}
	fmt.Println("  Ana, device 42, 10:00:", book(p2, 42, "10:00", "Ana"))
	close(s1) // node 1 leaves gracefully
	members = members[1:]
	p2.target = nil
	fmt.Printf("  node 1 left, the oldest is now node %d, the proxy buffers\n", oldest(members))
	r1, r2 := make(chan string, 1), make(chan string, 1)
	p2.Tell(Book{42, "10:00", "Ben", r1})
	p2.Tell(Book{43, "11:00", "Ben", r2})
	p3 := &Proxy{}
	r3 := make(chan string, 1)
	p3.Tell(Book{44, "12:00", "Cara", r3})
	s2 := startScheduler(oldest(members), j)
	p2.SetTarget(s2)
	p3.SetTarget(s2)
	fmt.Println("  Ben, device 42, 10:00:", <-r1)
	fmt.Println("  Ben, device 43, 11:00:", <-r2)
	fmt.Println("  Cara, device 44, 12:00:", <-r3)

	fmt.Println("\n== the other trap: traffic through the singleton")
	devices, handlerMs := 10000, 1 // 1 ms per reading is an assumption for the sum
	fmt.Printf("  %d readings a second x %d ms = %d s of work each second, one actor does 1 s\n",
		devices, handlerMs, devices*handlerMs/1000)

	fmt.Println("\n== partition, no resolver")
	j = &Journal{}
	pa := &Proxy{target: startScheduler(oldest([]int{1}), j)}
	pb := &Proxy{target: startScheduler(oldest([]int{2, 3}), j)}
	a := book(pa, 42, "10:00", "Ana")
	b := book(pb, 42, "10:00", "Ben")
	fmt.Println("  Ana, device 42, 10:00:", a)
	fmt.Println("  Ben, device 42, 10:00:", b)
	fmt.Printf("  journal holds %d bookings for device 42 at 10:00\n", len(j.All()))

	fmt.Println("\n== the fix: keep oldest, down-if-alone")
	j = &Journal{}
	fmt.Println("  node 1: oldest and alone -> downs itself, its scheduler stops")
	pb = &Proxy{target: startScheduler(oldest([]int{2, 3}), j)}
	fmt.Println("  Ana, device 42, 10:00:", book(pb, 42, "10:00", "Ana"))
	fmt.Println("  Ben, device 42, 10:00:", book(pb, 42, "10:00", "Ben"))
	fmt.Printf("  journal holds %d booking for device 42 at 10:00\n", len(j.All()))
}
