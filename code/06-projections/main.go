// A projection reads the journal in order and builds a read table. Where it saves its
// offset decides the guarantee, and an idempotent update makes replays harmless.
package main

import "fmt"

type Event struct {
	Seq    int
	Device int
	Online bool
}

var journal = []Event{
	{1, 41, true}, {2, 42, true}, {3, 42, false}, {4, 41, false}, {5, 42, true},
}

// Read is the read model: the table the dashboard queries, plus the saved offset.
type Read struct {
	online       map[int]bool
	offlineCount int
	lastSeq      map[int]int
	offset       int
}

func newRead() *Read { return &Read{online: map[int]bool{}, lastSeq: map[int]int{}} }

type Config struct {
	OffsetFirst bool // save the offset before the write
	CrashOnSeq  int  // the pod dies between the two steps of this event
	Idempotent  bool // keep the last event number per device
}

// run is the projection actor: one event at a time, from the saved offset. It returns
// false when the pod crashed.
func run(r *Read, c Config, upTo int) bool {
	mailbox := make(chan Event, len(journal))
	for _, e := range journal[r.offset:upTo] {
		mailbox <- e
	}
	close(mailbox)
	ok := make(chan bool)
	go func() {
		for e := range mailbox {
			if c.OffsetFirst {
				r.offset = e.Seq
				if e.Seq == c.CrashOnSeq {
					fmt.Printf("  crash: offset %d saved, event %d never written\n", e.Seq, e.Seq)
					ok <- false
					return
				}
				write(r, e, c)
			} else {
				write(r, e, c)
				if e.Seq == c.CrashOnSeq {
					fmt.Printf("  crash: event %d written, offset still %d\n", e.Seq, r.offset)
					ok <- false
					return
				}
				r.offset = e.Seq
			}
		}
		ok <- true
	}()
	return <-ok
}

func write(r *Read, e Event, c Config) {
	if c.Idempotent && r.lastSeq[e.Device] >= e.Seq {
		fmt.Printf("  event %d already applied to device %d, skipped\n", e.Seq, e.Device)
		return
	}
	r.lastSeq[e.Device] = e.Seq
	r.online[e.Device] = e.Online // an upsert: applying it twice changes nothing
	if !e.Online {
		r.offlineCount++ // adding: applying it twice counts twice
	}
}

func main() {
	fmt.Println("== events 1 to 3, then a restart that continues from the offset")
	r := newRead()
	run(r, Config{}, 3)
	fmt.Printf("  offset %d, table %v\n", r.offset, r.online)
	fmt.Printf("  dashboard: offline devices = device 42: %v\n", !r.online[42])
	run(r, Config{}, 4)
	fmt.Printf("  after restart: offset %d, device 41 online = %v\n", r.offset, r.online[41])
	fmt.Printf("  event 5 not read yet: device 42 online = %v, for now\n", r.online[42])

	fmt.Println("== offset before the write: at most once")
	r = newRead()
	run(r, Config{}, 3)
	run(r, Config{OffsetFirst: true, CrashOnSeq: 4}, 5)
	run(r, Config{OffsetFirst: true}, 5)
	fmt.Printf("  restarted at offset 4, now offset %d. device 41 online = %v, wrong for ever\n", r.offset, r.online[41])

	for _, idem := range []bool{false, true} {
		if idem {
			fmt.Println("== offset after the write, last event number per device")
		} else {
			fmt.Println("== offset after the write: at least once, a count that adds")
		}
		r = newRead()
		run(r, Config{Idempotent: idem}, 3)
		fmt.Printf("  offset %d, offline count %d\n", r.offset, r.offlineCount)
		run(r, Config{CrashOnSeq: 4, Idempotent: idem}, 4)
		fmt.Printf("  offline count %d\n", r.offlineCount)
		run(r, Config{Idempotent: idem}, 4)
		fmt.Printf("  after replay: offset %d, device 41 online = %v, offline count %d, really offline 2\n", r.offset, r.online[41], r.offlineCount)
	}
}
