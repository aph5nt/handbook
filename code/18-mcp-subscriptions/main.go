// Resource subscriptions: a short notice with the URI only, then the host
// reads again. A debounce stops a flood. Run: go run .
package main

import "fmt"

type Server struct {
	head       int
	subs       map[string]bool
	notify     func(uri string)
	minBlocks  int // send a notice only after this many blocks
	lastNotice int
	Notices    int
}

func (s *Server) Capabilities() string { return `{"resources":{"subscribe":true}}` }

func (s *Server) Subscribe(uri string) { s.subs[uri] = true }

func (s *Server) Read(uri string) string { return fmt.Sprintf("head: block %d", s.head) }

// NewBlock is the indexer writing one block.
func (s *Server) NewBlock() {
	s.head++
	const uri = "graph://robinhood/head"
	if !s.subs[uri] {
		return
	}
	if s.head-s.lastNotice < s.minBlocks {
		return // the debounce: too soon, the next notice will cover it
	}
	s.lastNotice = s.head
	s.Notices++
	s.notify(uri) // notifications/resources/updated, URI only, no data
}

func newServer(minBlocks int) *Server {
	return &Server{head: 9506, lastNotice: 9506, subs: map[string]bool{}, minBlocks: minBlocks}
}

func main() {
	s := newServer(1)
	s.notify = func(uri string) {
		fmt.Println("  <- notifications/resources/updated", uri)
		fmt.Println("  -> resources/read", uri, "<-", s.Read(uri))
	}
	fmt.Println("0 capabilities:", s.Capabilities())
	s.Subscribe("graph://robinhood/head")
	fmt.Println("1 resources/subscribe graph://robinhood/head")
	s.NewBlock()

	reads := 0
	flood := newServer(1)
	flood.Subscribe("graph://robinhood/head")
	flood.notify = func(uri string) { reads++ }
	for i := 0; i < 600; i++ {
		flood.NewBlock()
	}
	fmt.Println("5 no debounce, 600 blocks:", flood.Notices, "notices,", reads, "reads")

	reads = 0
	calm := newServer(100)
	calm.Subscribe("graph://robinhood/head")
	calm.notify = func(uri string) { reads++ }
	for i := 0; i < 600; i++ {
		calm.NewBlock()
	}
	fmt.Println("5 notice every 100 blocks, 600 blocks:", calm.Notices, "notices,", reads, "reads")
}
