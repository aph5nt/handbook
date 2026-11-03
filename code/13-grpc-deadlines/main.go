// A sync service calls an indexer, which calls a slow node. Replays the
// lesson 13-grpc-deadlines. The hops are net/http handlers and the deadline
// travels in a grpc-timeout style header, because the program uses the
// standard library only. gRPC does the same with its own header. One story
// second is one millisecond here, so the demo takes less than a second.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const second = time.Millisecond

// Node takes delay to answer, unless the caller goes away first.
type Node struct {
	delay             time.Duration
	finished, stopped atomic.Int64
	running, peak     atomic.Int64
}

func (n *Node) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	now := n.running.Add(1)
	defer n.running.Add(-1)
	for {
		p := n.peak.Load()
		if now <= p || n.peak.CompareAndSwap(p, now) {
			break
		}
	}
	select {
	case <-time.After(n.delay):
		n.finished.Add(1)
	case <-r.Context().Done():
		n.stopped.Add(1)
	}
}

// indexer calls the node. It uses 2 seconds, then passes down what is left.
func indexer(nodeURL string, passDown bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background() // no link to the caller: the cancel stops here
		if passDown {
			ctx = r.Context()
			if ms, err := strconv.Atoi(r.Header.Get("Grpc-Timeout")); err == nil {
				remaining := time.Duration(ms)*second - 2*second
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, remaining)
				defer cancel()
			}
		}
		req, _ := http.NewRequestWithContext(ctx, "GET", nodeURL, nil)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, "DEADLINE_EXCEEDED", http.StatusGatewayTimeout)
			return
		}
		res.Body.Close()
	}
}

// streamBlocks is the sync service's call, with or without a deadline.
func streamBlocks(ctx context.Context, url string, deadline time.Duration) error {
	if deadline > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, deadline)
		defer cancel()
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	if deadline > 0 {
		req.Header.Set("Grpc-Timeout", strconv.Itoa(int(deadline/second)))
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusGatewayTimeout {
		return errDeadline
	}
	return nil
}

var errDeadline = errors.New("DEADLINE_EXCEEDED")

func main() {
	fmt.Println("--- no deadline")
	slow := &Node{delay: 120 * second}
	nodeSrv := httptest.NewServer(slow)
	defer nodeSrv.Close()
	idx := httptest.NewServer(indexer(nodeSrv.URL, false))
	defer idx.Close()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ { // more ranges, each call waits
		wg.Add(1)
		go func() {
			defer wg.Done()
			streamBlocks(context.Background(), idx.URL, 0)
		}()
	}
	wg.Wait()
	fmt.Println("calls waiting on the node at once:", slow.peak.Load())
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(10 * second); cancel() }()
	before := slow.finished.Load()
	err := streamBlocks(ctx, idx.URL, 0)
	time.Sleep(150 * second)
	fmt.Println("operator cancels:", errors.Is(err, context.Canceled), "| node finished the dead call anyway:", slow.finished.Load()-before == 1)

	fmt.Println("--- deadlines passed down")
	node2 := &Node{delay: 60 * second}
	node2Srv := httptest.NewServer(node2)
	defer node2Srv.Close()
	idx2 := httptest.NewServer(indexer(node2Srv.URL, true))
	defer idx2.Close()
	err = streamBlocks(context.Background(), idx2.URL, 30*second)
	time.Sleep(40 * second)
	fmt.Println("deadline 30 s, indexer used 2, node gets 28:", err, "| node stopped:", node2.stopped.Load(), "finished:", node2.finished.Load())

	fast := &Node{delay: 1 * second}
	fastSrv := httptest.NewServer(fast)
	defer fastSrv.Close()
	idx3 := httptest.NewServer(indexer(fastSrv.URL, true))
	defer idx3.Close()
	err = streamBlocks(context.Background(), idx3.URL, 30*second)
	fmt.Println("retry on the second node: err", err)

	ctx, cancel = context.WithCancel(context.Background())
	node3 := &Node{delay: 200 * second}
	node3Srv := httptest.NewServer(node3)
	defer node3Srv.Close()
	idx4 := httptest.NewServer(indexer(node3Srv.URL, true))
	defer idx4.Close()
	go func() { time.Sleep(10 * second); cancel() }()
	err = streamBlocks(ctx, idx4.URL, 0)
	time.Sleep(30 * second)
	fmt.Println("operator cancels:", errors.Is(err, context.Canceled), "| node stopped:", node3.stopped.Load())
}
