// Dead-letter exchange (RabbitMQ) and redrive (Amazon SQS).
//
// Go structs stand in for both brokers. Time is a counter in seconds, so
// nothing sleeps. RabbitMQ's real rule is modelled: a message expires only
// when it reaches the head of its queue. Deterministic.
package main

import "fmt"

// ---- RabbitMQ: reject -> retry queue with a TTL -> back -> park ----

type Msg struct {
	ID    string
	TTL   int // seconds, 0 = none
	Death int // x-death count
	PutAt int
}

const maxDeaths = 3

// reject with requeue=false: the broker sends m to the queue's dead-letter exchange.
func reject(m Msg, requeue bool) {
	if !requeue {
		fmt.Printf("%s rejected, requeue=false: dead-lettered\n", m.ID)
	}
}

// publish puts m on a queue.
func publish(queue string, m Msg) {
	fmt.Printf("%s published to %s\n", m.ID, queue)
}

func rabbitRetry() {
	now := 0
	m := Msg{ID: "m3"}
	for {
		reject(m, false) // requeue=false: the broker dead-letters it
		m.Death++
		if m.Death >= maxDeaths {
			fmt.Printf("t=%ds failure %d: x-death count %d\n", now, m.Death, m.Death)
			publish("decode.dead", m)
			return
		}
		fmt.Printf("t=%ds failure %d: dead-lettered to decode.retry.10s\n", now, m.Death)
		now += 10 // nobody reads the retry queue: it waits for its TTL
		fmt.Printf("t=%ds expired: its dead-letter exchange routes it back to decode\n", now)
	}
}

// expiry returns when each message leaves a queue that only checks its head.
func expiry(q []Msg) map[string]int {
	out := map[string]int{}
	now := 0
	for _, m := range q {
		due := m.PutAt + m.TTL
		if due > now {
			now = due // the head blocks everything behind it
		}
		out[m.ID] = now
	}
	return out
}

// ---- SQS: receive count -> redrive policy -> DLQ ----

const maxReceive = 4

func sqsReceive() int {
	receives := 0
	for {
		receives++ // received, failed, not deleted: visible again later
		if receives >= maxReceive {
			return receives // SQS moves it to decode-dlq
		}
	}
}

func main() {
	fmt.Println("== Tab 1: RabbitMQ ==")
	rabbitRetry()
	left := expiry([]Msg{{ID: "60 s msg", TTL: 60}, {ID: "5 s msg", TTL: 5}})
	fmt.Printf("mixed TTLs in one queue: 60 s msg leaves at %ds, 5 s msg leaves at %ds\n", left["60 s msg"], left["5 s msg"])
	left = expiry([]Msg{{ID: "5 s msg", TTL: 5}})
	fmt.Printf("one queue per delay: 5 s msg leaves at %ds\n", left["5 s msg"])

	fmt.Println("== Tab 2: SQS ==")
	fmt.Printf("m3: moved to decode-dlq after %d receives\n", sqsReceive())
	parked := 5000
	wasted := parked * maxReceive // 5,000 x 4 = 20,000
	fmt.Printf("redrive before the fix: %d messages x %d failed receives = %d, all back in the DLQ\n", parked, maxReceive, wasted)
	rate := 100
	seconds := parked / rate // 5,000 at 100 a second = 50 s
	fmt.Printf("after the fix, redrive at %d a second: %d messages in %d seconds\n", rate, parked, seconds)
}
