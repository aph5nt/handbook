// Fan-out: one block-indexed message copied to a queue per subscriber,
// then the same story with a push to HTTP endpoints and no queue.
//
// A buffered Go channel stands in for each broker queue (RabbitMQ or SQS):
// it holds messages until its own consumer reads them. Publish plays the
// fanout exchange. The run is single-threaded so the numbers are exact.
package main

import "fmt"

// Msg is one block-indexed event.
type Msg struct{ Block int }

// Exchange is a fanout exchange with one bound queue per subscriber.
type Exchange struct {
	names  []string
	queues map[string]chan Msg
}

func NewExchange() *Exchange { return &Exchange{queues: map[string]chan Msg{}} }

// Bind adds one queue for one subscriber. The publisher never changes.
func (x *Exchange) Bind(name string, size int) {
	x.names = append(x.names, name)
	x.queues[name] = make(chan Msg, size)
}

// Publish copies the message into every bound queue: one in, N out.
func (x *Exchange) Publish(m Msg) {
	for _, n := range x.names {
		x.queues[n] <- m // a full queue would block here: size it for an outage
	}
}

// Drain lets one consumer read and acknowledge all it has waiting.
// A consumer that is down simply does not call it: its messages wait.
func (x *Exchange) Drain(name string) int {
	q, n := x.queues[name], 0
	for len(q) > 0 {
		<-q // handle, then ack: removed from this queue only
		n++
	}
	return n
}

// ---- push with no queue: SNS straight to HTTP endpoints ----

const maxRetries = 3 // this example's policy: retry after 1 s, 2 s, 4 s

// Endpoint is down from DownFrom (inclusive) to DownTo (exclusive), in seconds.
type Endpoint struct {
	Name             string
	DownFrom, DownTo int
}

func (e Endpoint) Up(t int) bool { return t < e.DownFrom || t >= e.DownTo }

// push tries one endpoint, retrying with backoff. False means the message is dropped.
func push(e Endpoint, sentAt int) bool {
	t := sentAt
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if e.Up(t) {
			return true
		}
		t += 1 << attempt // wait 1 s, 2 s, 4 s before the next try
	}
	return false // retries ran out and nothing holds the message
}

func main() {
	fmt.Println("== Tab 1: a queue per subscriber ==")
	x := NewExchange()
	for _, n := range []string{"graph", "price", "alert"} {
		x.Bind(n, 10000)
	}
	x.Publish(Msg{9506})
	fmt.Printf("step 2: published block 9506 once, queue depths graph=%d price=%d alert=%d\n",
		len(x.queues["graph"]), len(x.queues["price"]), len(x.queues["alert"]))
	x.Drain("graph")
	x.Drain("price")
	// The alert rule is down for an hour. One block-indexed every 10 s in this example.
	for b := 9507; b < 9506+360; b++ {
		x.Publish(Msg{b})
		x.Drain("graph")
		x.Drain("price")
	}
	fmt.Printf("step 4: alert rule down an hour: graph=%d price=%d alert=%d waiting\n",
		len(x.queues["graph"]), len(x.queues["price"]), len(x.queues["alert"]))
	fmt.Printf("step 5: alert rule back, handled %d, lost 0\n", x.Drain("alert"))
	x.Bind("audit", 10000)
	x.Publish(Msg{9866})
	fmt.Printf("later: bound an audit queue, block 9866 copied to %d queues\n", len(x.names))

	fmt.Println("== Tab 2: push with no queue ==")
	alert := Endpoint{"alert", 0, 3600} // down for the first hour
	lost := 0
	for t := 0; t < 3600; t += 10 {
		if !push(alert, t) {
			lost++
		}
	}
	fmt.Printf("step 3: each push tries %d times over %d s, the endpoint is down 3600 s\n", maxRetries+1, 1+2+4)
	fmt.Printf("step 4: lost %d of 360 messages for the alert rule\n", lost)
	// The fix is the queue tab: SNS copies into one SQS
	// queue per consumer, and each consumer drains its own.
	x = NewExchange()
	x.Bind("alert", 10000)
	for t := 0; t < 3600; t += 10 {
		msg := Msg{9506 + t/10}
		x.Publish(msg)
	}
	drained := x.Drain("alert") // after the deploy: nothing lost
	fmt.Printf("step 5: fix = an SQS queue per subscriber: alert drains %d of 360 after the deploy, lost 0\n", drained)
}
