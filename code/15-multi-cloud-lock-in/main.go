// Keep the door open with a small port: the service calls one interface,
// and only one adapter knows the cloud. Then what two clouds at once cost.
// Prices are made-up examples.
package main

import "fmt"

// Queue is the port. The service knows only this.
type Queue interface {
	Publish(topic, msg string) error
	Name() string
}

// CloudAQueue is the adapter for cloud A's own queue.
type CloudAQueue struct{}

func (CloudAQueue) Publish(topic, msg string) error { return nil } // cloud A's client here
func (CloudAQueue) Name() string                    { return "cloud A queue" }

// KafkaQueue is the adapter for Kafka, on any Kubernetes.
type KafkaQueue struct{}

func (KafkaQueue) Publish(topic, msg string) error { return nil } // a Kafka client here
func (KafkaQueue) Name() string                    { return "Kafka" }

// Indexer takes a Queue, never a cloud client.
type Indexer struct{ Out Queue }

func (ix Indexer) Flush(block int) error {
	return ix.Out.Publish("blocks", fmt.Sprint(block))
}

// moveCost counts the files a move touches.
func moveCost(callSites int, hasPort bool) int {
	if hasPort {
		return 1 // one adapter
	}
	return callSites // each one is rewritten and retested
}

const egressPerGB = 0.09

// crossCloud is the monthly bill for querying data on the other cloud.
func crossCloud(gbPerDay float64) float64 { return gbPerDay * 30 * egressPerGB }

func main() {
	for _, q := range []Queue{CloudAQueue{}, KafkaQueue{}} {
		ix := Indexer{Out: q}
		fmt.Printf("indexer flushes block 9506000 through %s: err %v\n", q.Name(), ix.Flush(9506000))
	}
	fmt.Printf("move to cloud B, team 1 (no port, 240 call sites): %d files\n", moveCost(240, false))
	fmt.Printf("move to cloud B, team 2 (Kafka behind a port): %d file\n", moveCost(240, true))
	fmt.Printf("two clouds at once, 2,000 GB a day across: %.0f a month in egress\n", crossCloud(2000))
}
