// Passivation: the shard stops an entity that is idle past the timeout, buffers messages
// that arrive while it stops, and starts it again from the journal. A simulated clock in
// minutes makes the run repeat exactly.
package main

import (
	"fmt"
	"sort"
)

const slots = 8

type Msg struct {
	Device int
	Text   string
}
type Tick struct{ Minute int }
type Stopped struct{ Device int }
type NodeRestart struct{}

// Shard is an actor: one goroutine, one mailbox. It owns the entity table.
type Shard struct {
	mailbox    chan any
	active     map[int]int   // device -> minute of its last message
	stopping   map[int]bool  // asked to passivate
	buffer     map[int][]Msg // messages for an entity that is stopping
	idleLimit  int           // minutes. 0 means passivation is off
	remembered map[int]bool
	remember   bool
	now        int
	recoveries int
	crashed    bool
}

func newShard(idleLimit int, remember bool) *Shard {
	s := &Shard{mailbox: make(chan any, 64), active: map[int]int{}, stopping: map[int]bool{},
		buffer: map[int][]Msg{}, idleLimit: idleLimit, remembered: map[int]bool{}, remember: remember}
	go s.run()
	return s
}

func (s *Shard) recover(d int, why string) {
	s.recoveries++
	fmt.Printf("  %02d:%02d device %d: %s, snapshot + events, %d of %d slots\n",
		9+s.now/60, s.now%60, d, why, len(s.active), slots)
}

func (s *Shard) start(d int) {
	if len(s.active) >= slots {
		s.crashed = true
		fmt.Printf("  device %d: no slot left, node out of memory, every actor lost\n", d)
		s.active = map[int]int{}
		return
	}
	s.active[d] = s.now
	if s.remember {
		s.remembered[d] = true
	}
	fmt.Printf("  %02d:%02d device %d: started, %d of %d slots\n", 9+s.now/60, s.now%60, d, len(s.active), slots)
}

func (s *Shard) run() {
	for m := range s.mailbox {
		switch v := m.(type) {
		case Msg:
			if s.crashed {
				continue
			}
			if s.stopping[v.Device] { // stopping: hold it, lose nothing
				s.buffer[v.Device] = append(s.buffer[v.Device], v)
				fmt.Printf("  device %d: stopping, buffer %q\n", v.Device, v.Text)
				continue
			}
			if _, up := s.active[v.Device]; !up {
				s.start(v.Device)
			}
			s.active[v.Device] = s.now
		case Tick:
			s.now = v.Minute
			if s.idleLimit == 0 {
				continue
			}
			devices := []int{}
			for d := range s.active {
				devices = append(devices, d)
			}
			sort.Ints(devices) // a fixed order, so the output repeats
			for _, d := range devices {
				last := s.active[d]
				if s.now-last >= s.idleLimit && !s.stopping[d] {
					s.stopping[d] = true
					fmt.Printf("  %02d:%02d device %d: idle %d min, passivate\n", 9+s.now/60, s.now%60, d, s.now-last)
				}
			}
		case Stopped:
			delete(s.active, v.Device)
			delete(s.stopping, v.Device)
			fmt.Printf("  device %d: stopped, %d of %d slots, state safe in the journal\n", v.Device, len(s.active), slots)
			if buf := s.buffer[v.Device]; len(buf) > 0 { // a buffered message restarts it
				s.active[v.Device] = s.now
				s.recover(v.Device, "recovering for a buffered message")
				for _, b := range buf {
					fmt.Printf("  device %d: handled %q\n", v.Device, b.Text)
				}
				delete(s.buffer, v.Device)
			}
		case NodeRestart:
			s.active = map[int]int{}
			for d := range s.remembered { // no message waits for any of them
				s.active[d] = 0
				s.recoveries++
			}
		case chan string:
			v <- fmt.Sprintf("%d of %d slots, recoveries: %d", len(s.active), slots, s.recoveries)
		}
	}
}

func (s *Shard) status() string {
	r := make(chan string)
	s.mailbox <- r
	return <-r
}

func main() {
	fmt.Println("== passivation, idle timeout 2 min")
	s := newShard(2, false)
	s.mailbox <- Msg{42, "reading"}
	s.mailbox <- Tick{1}
	s.mailbox <- Msg{42, "reading"} // its last message, at 09:01
	s.mailbox <- Msg{17, "reading"}
	s.mailbox <- Msg{8, "reading"}
	s.mailbox <- Tick{2}
	s.mailbox <- Msg{17, "reading"} // these two stay busy
	s.mailbox <- Msg{8, "reading"}
	s.mailbox <- Tick{3}                     // device 42 idle for 2 min
	s.mailbox <- Msg{42, "SetInterval 30 s"} // arrives while it stops
	s.mailbox <- Stopped{42}
	fmt.Println(" ", s.status())

	fmt.Println("== passivation off, one reading an hour per device")
	s = newShard(0, false)
	for _, d := range []int{42, 17, 8, 99, 3, 61, 70, 12, 55} {
		s.mailbox <- Msg{d, "reading"}
	}
	fmt.Println(" ", s.status())

	fmt.Println("== the fix on the same traffic, only 2 devices sent recently")
	s = newShard(2, false)
	for _, d := range []int{17, 8, 3, 61, 70, 12} {
		s.mailbox <- Msg{d, "reading"}
	}
	s.mailbox <- Tick{2}
	for _, d := range []int{17, 8, 3, 61, 70, 12} {
		s.mailbox <- Stopped{d}
	}
	s.mailbox <- Msg{42, "reading"}
	s.mailbox <- Msg{99, "reading"}
	fmt.Println(" ", s.status())

	fmt.Println("== remember entities for every device, node restart")
	s = newShard(0, true)
	for _, d := range []int{42, 17, 8, 99} {
		s.mailbox <- Msg{d, "reading"}
	}
	s.mailbox <- NodeRestart{}
	fmt.Println("  after the restart, no message waiting:", s.status())
	fmt.Println("  with 1,000,000 devices: 1,000,000 recoveries in the same minute")

	fmt.Println("== the fix: remember only the entity with a timer")
	s = newShard(0, true)
	s.mailbox <- Msg{42, "start the timer"}
	s.mailbox <- NodeRestart{}
	fmt.Println(" ", s.status())
}
