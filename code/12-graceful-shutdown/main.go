// Command gracefulshutdown replays the lesson in two parts. The timeline uses a virtual
// clock, so its numbers are the same on every run: six queries in flight, SIGTERM at 0 s,
// a 30 s grace period. Then a real demo: a real HTTP server, a real SIGTERM sent to this
// process, one request in flight that still gets its answer.
package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	grace         = 30 // seconds Kubernetes waits between SIGTERM and SIGKILL
	preStop       = 5  // seconds the pod keeps serving after it fails readiness
	balancerDrops = 3  // seconds until the balancer has dropped the pod
	queryTimeout  = 25 // seconds every in-flight call may take: under grace - preStop
	flush         = 1  // seconds to flush logs and close connections
)

// Drain returns when the pod exits and how many queries it lost.
// Each entry of inFlight is the seconds that query still needs after SIGTERM.
func Drain(inFlight []int, timeout int) (exitAt, finished, lost int, killed bool) {
	last := preStop // the pod stops accepting after the preStop sleep
	for _, need := range inFlight {
		end := need
		if timeout > 0 && end > timeout {
			end = timeout // the query's own timeout ends it with an error answer
			lost++
		} else {
			finished++
		}
		if end > last {
			last = end
		}
	}
	exitAt = last + flush
	if exitAt > grace { // SIGKILL cannot be caught
		return grace, finished - countOver(inFlight, grace), lost + countOver(inFlight, grace), true
	}
	return exitAt, finished, lost, false
}

func countOver(inFlight []int, at int) int {
	n := 0
	for _, need := range inFlight {
		if need > at {
			n++
		}
	}
	return n
}

// NoHandler is the process with no signal handler: it ends at once on SIGTERM.
// Everything in flight is lost, and so are the queries sent until the balancer notices.
func NoHandler(inFlight, newPerSecond int) (lost, refused int) {
	return inFlight, newPerSecond * balancerDrops
}

// Real runs a real server and sends SIGTERM to this very process.
func Real() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer stop()

	var ready atomic.Bool
	ready.Store(true)
	mux := http.NewServeMux()
	mux.HandleFunc("/ready", func(w http.ResponseWriter, _ *http.Request) {
		if !ready.Load() {
			w.WriteHeader(http.StatusServiceUnavailable) // the balancer takes the pod out
		}
	})
	mux.HandleFunc("/query", func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond) // a query that is in flight during SIGTERM
		fmt.Fprint(w, "answer")
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	url := "http://" + ln.Addr().String()

	answer := make(chan string)
	go func() {
		resp, err := http.Get(url + "/query")
		if err != nil {
			answer <- "failed: " + err.Error()
			return
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		answer <- fmt.Sprintf("%d %s", resp.StatusCode, b)
	}()
	time.Sleep(100 * time.Millisecond) // the query is now in flight

	syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
	<-ctx.Done()
	fmt.Println("real: SIGTERM caught")

	ready.Store(false) // 1. fail readiness, keep serving
	resp, err := http.Get(url + "/ready")
	if err != nil {
		panic(err)
	}
	resp.Body.Close()
	fmt.Println("real: /ready now answers", resp.StatusCode)

	drain, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = srv.Shutdown(drain) // 2. stop accepting, wait for in-flight work under a deadline
	fmt.Println("real: in-flight query got:", <-answer)
	fmt.Println("real: drained, error:", err)
}

func main() {
	queries := []int{1, 2, 4, 6, 9, 12} // seconds each of the six still needs after SIGTERM

	exit, done, lost, killed := Drain(queries, queryTimeout)
	fmt.Printf("drain: %d finished, %d lost, exit 0 at %d s, %d s of grace unused, killed: %v\n",
		done, lost, exit, grace-exit, killed)

	lost, refused := NoHandler(6, 20)
	fmt.Printf("no handler: %d queries lost in flight, %d refused until the balancer notices\n", lost, refused)

	exit, done, lost, killed = Drain([]int{1, 2, 4, 6, 9, 40}, 0)
	fmt.Printf("no deadline, one 40 s walk: %d finished, %d cut off, ends at %d s, killed: %v\n", done, lost, exit, killed)

	exit, done, lost, killed = Drain([]int{1, 2, 4, 6, 9, 40}, queryTimeout)
	fmt.Printf("with a %d s query timeout: %d finished, %d timed out, exit 0 at %d s, killed: %v\n", queryTimeout, done, lost, exit, killed)

	fmt.Println()
	Real()
}
