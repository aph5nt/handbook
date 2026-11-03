// Progress and cancellation: a token ties progress to a request, and a cancel
// must reach the running statement. Run: go run .
package main

import (
	"context"
	"fmt"
)

type Proxy struct {
	running map[int]context.CancelFunc // request id -> stop the statement
	honest  bool
	step    chan struct{} // the demo advances the fake statement
	out     chan string   // what the proxy writes to the host
}

// call runs a long statement and reports progress under the client's token.
func (p *Proxy) call(id int, token string) {
	ctx, cancel := context.WithCancel(context.Background())
	p.running[id] = cancel
	go func() {
		for pct := 30; pct <= 90; pct += 30 {
			select {
			case <-ctx.Done():
				p.out <- fmt.Sprintf("db: statement for id %d stopped after %d%%", id, pct-30)
				return // no response is sent for a cancelled request
			case <-p.step:
				p.out <- fmt.Sprintf(`{"method":"notifications/progress","params":{"progressToken":%q,"progress":%d,"total":100}}`, token, pct)
			}
		}
		p.out <- fmt.Sprintf(`{"id":%d,"result":{}}`, id)
	}()
}

// onCancelled handles notifications/cancelled. It has no id and gets no reply.
func (p *Proxy) onCancelled(id int) {
	if !p.honest {
		fmt.Printf("  (bug) cancel for id %d accepted, statement keeps running\n", id)
		return
	}
	if stop, ok := p.running[id]; ok {
		stop()
		delete(p.running, id)
	}
}

// The host ignores a response whose id it already cancelled.
func hostReceive(cancelled map[int]bool, id int) string {
	if cancelled[id] {
		return fmt.Sprintf("late result for id %d ignored", id)
	}
	return "shown to the model"
}

func newProxy(honest bool) *Proxy {
	return &Proxy{running: map[int]context.CancelFunc{}, honest: honest, step: make(chan struct{}), out: make(chan string)}
}

func main() {
	p := newProxy(true)
	fmt.Println("0 tools/call id 7, _meta.progressToken p1")
	p.call(7, "p1")
	for i := 0; i < 2; i++ {
		p.step <- struct{}{}
		fmt.Println("1", <-p.out)
	}
	fmt.Println("2 user presses stop: notifications/cancelled requestId 7")
	p.onCancelled(7)
	fmt.Println("3", <-p.out)
	fmt.Println("4", hostReceive(map[int]bool{7: true}, 7))

	bad := newProxy(false)
	bad.call(8, "p2")
	fmt.Println("5 notifications/cancelled requestId 8")
	bad.onCancelled(8)
	bad.step <- struct{}{}
	fmt.Println("  still running:", <-bad.out)
}
