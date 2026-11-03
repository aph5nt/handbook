// Delivery guarantees: at most once loses, at least once duplicates, and an idempotent
// receiver makes the duplicate harmless. Network faults are scripted so the run repeats.
package main

import (
	"fmt"
	"time"
)

type Reading struct{ C float64 }
type Command struct {
	ID       int
	Set, Add int // Set an absolute interval, or Add to it
}
type Ack struct{ ID int }
type Get struct{ Reply chan string }

// Device is the actor. rememberIDs turns on the seen set, the idempotent receiver.
func device(mailbox chan any, ackTo chan Ack, rememberIDs bool, start int) {
	interval, last := start, 21.3
	seen := map[int]bool{}
	for m := range mailbox {
		switch v := m.(type) {
		case Reading:
			last = v.C
		case Command:
			if rememberIDs && seen[v.ID] {
				fmt.Printf("  device 42: id %d already seen, no change, ack again\n", v.ID)
			} else {
				if v.Add != 0 {
					interval += v.Add
				} else {
					interval = v.Set
				}
				seen[v.ID] = true
				fmt.Printf("  device 42: applied id %d, interval %d s\n", v.ID, interval)
			}
			if ackTo != nil {
				ackTo <- Ack{v.ID}
			}
		case Get:
			v.Reply <- fmt.Sprintf("last %.1f C, interval %d s", last, interval)
		}
	}
}

func state(mb chan any) string {
	r := make(chan string)
	mb <- Get{r}
	return <-r
}

// Journal stores what the sender promised. It survives a sender crash.
type Journal map[int]string

// sendAtLeastOnce stores the command, sends it, and resends until an ack comes.
// firstCopyHeld scripts the blip: the first copy is held up and arrives after the resend.
func sendAtLeastOnce(j Journal, cmd Command, mb chan any, acks chan Ack, firstCopyHeld bool) {
	j[cmd.ID] = "pending" // store first
	fmt.Printf("  sender: journal id %d pending\n", cmd.ID)
	var held chan struct{}
	if firstCopyHeld {
		held = make(chan struct{})
		go func() { <-held; fmt.Println("  network: the held-up copy arrives late"); mb <- cmd }()
		fmt.Printf("  sender: sent id %d, held up by the network\n", cmd.ID)
	} else {
		mb <- cmd
	}
	for {
		select {
		case a := <-acks:
			j[a.ID] = "done"
			fmt.Printf("  sender: ack %d, journal id %d done\n", a.ID, a.ID)
			if held != nil {
				close(held) // now the late copy lands
				<-acks      // and is acked again
			}
			return
		case <-time.After(20 * time.Millisecond):
			fmt.Printf("  sender: no ack before the timer, resend id %d\n", cmd.ID)
			mb <- cmd
		}
	}
}

func main() {
	fmt.Println("== at most once")
	mb := make(chan any, 8)
	go device(mb, nil, false, 60)
	mb <- Reading{21.5}
	fmt.Println("  device 42:", state(mb))
	fmt.Println("  reading 21.7 C: dropped by the network, nobody notices")
	mb <- Reading{21.9}
	fmt.Println("  command set interval 30 s: dropped by the network, sender thinks it is done")
	fmt.Println("  device 42:", state(mb))

	fmt.Println("== at least once, id 7, set interval 30 s, seen set on")
	acks := make(chan Ack, 8)
	mb = make(chan any, 8)
	go device(mb, acks, true, 60)
	j := Journal{}
	sendAtLeastOnce(j, Command{ID: 7, Set: 30}, mb, acks, true)

	fmt.Println("== the sender crashes before any ack")
	j = Journal{7: "pending"} // what the journal holds after the crash
	fmt.Println("  sender restarted: journal has id 7 pending, resend")
	mb = make(chan any, 8)
	acks = make(chan Ack, 8)
	go device(mb, acks, true, 60)
	sendAtLeastOnce(j, Command{ID: 7, Set: 30}, mb, acks, false)

	fmt.Println("== relative command, id 8, add 5 s, no seen set")
	mb = make(chan any, 8)
	acks = make(chan Ack, 8)
	go device(mb, acks, false, 30)
	sendAtLeastOnce(Journal{}, Command{ID: 8, Add: 5}, mb, acks, true)
	fmt.Println("  device 42:", state(mb), "<- should be 35 s")

	fmt.Println("== fix 1: send the absolute value")
	mb = make(chan any, 8)
	acks = make(chan Ack, 8)
	go device(mb, acks, false, 30)
	sendAtLeastOnce(Journal{}, Command{ID: 8, Set: 35}, mb, acks, true)
	fmt.Println("  device 42:", state(mb))

	fmt.Println("== fix 2: remember the ids")
	mb = make(chan any, 8)
	acks = make(chan Ack, 8)
	go device(mb, acks, true, 30)
	sendAtLeastOnce(Journal{}, Command{ID: 8, Add: 5}, mb, acks, true)
	fmt.Println("  device 42:", state(mb))
}
