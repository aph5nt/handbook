// Queue versus topic: the same broker shapes as in the lesson.
//
// A Go slice stands in for the broker's storage. Queue.Take removes the
// message, so only one worker ever gets it. Topic.Publish appends a copy to
// every subscription, so every subscriber gets it. Single-threaded, so the
// printed numbers are exact.
package main

import (
	"fmt"
	"hash/fnv"
)

// Queue: point-to-point. Take hands the head to one worker and removes it.
type Queue struct{ msgs []string }

func (q *Queue) Put(m string) { q.msgs = append(q.msgs, m) }

func (q *Queue) Take() (string, bool) {
	if len(q.msgs) == 0 {
		return "", false
	}
	m := q.msgs[0]
	q.msgs = q.msgs[1:] // gone for every other worker
	return m, true
}

// Topic: publish-subscribe. Each subscription has its own copy of the stream.
type Topic struct {
	subs map[string][]map[string]int
	keys []string
}

func (t *Topic) Subscribe(name string) {
	t.subs[name] = nil
	t.keys = append(t.keys, name)
}

func (t *Topic) Publish(m map[string]int) {
	for _, s := range t.keys {
		t.subs[s] = append(t.subs[s], m) // one copy per subscriber
	}
}

// workerFor routes a key to one worker: the fix for order on a queue.
func workerFor(key string, workers int) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32()) % workers
}

func main() {
	fmt.Println("== Tab 1: queue ==")
	q := &Queue{}
	for i := 1; i <= 3; i++ {
		q.Put(fmt.Sprintf("job %d", i))
	}
	done := map[string]int{}
	// each worker, in turn
	w := 0
	for {
		m, ok := q.Take()
		if !ok {
			break
		}
		w++
		done[m]++ // resize, then ack
		fmt.Printf("worker %d takes %s, %d left on the queue\n", w, m, len(q.msgs))
	}
	fmt.Printf("each job done once: job 1=%d job 2=%d job 3=%d\n", done["job 1"], done["job 2"], done["job 3"])
	for i := 1; i <= 30; i++ {
		q.Put(fmt.Sprintf("photo %d", i))
	}
	per := make([]int, 4)
	for w := 0; ; w = (w + 1) % 4 { // four workers, each takes the next free job
		if _, ok := q.Take(); !ok {
			break
		}
		per[w]++
	}
	fmt.Printf("30 photos, 4 workers: %v jobs each, total %d\n", per, per[0]+per[1]+per[2]+per[3])

	fmt.Println("== Tab 2: topic ==")
	t := &Topic{subs: map[string][]map[string]int{}}
	for _, s := range []string{"graph sync", "price service", "alert rule"} {
		t.Subscribe(s)
	}
	t.Publish(map[string]int{"block": 101200000, "gas_used": 21000})
	for _, s := range t.keys {
		fmt.Printf("%s has %d copy of block %d\n", s, len(t.subs[s]), t.subs[s][0]["block"])
	}
	t.Publish(map[string]int{"block": 101200001}) // gas_used removed
	// the alert rule reads its own copy
	msg := t.subs["alert rule"][1]
	if _, ok := msg["gas_used"]; !ok {
		fmt.Println("block 101200001 has no gas_used: the alert rule fails")
	}

	fmt.Println("== Tab 3: traps ==")
	jobTopic := &Topic{subs: map[string][]map[string]int{}}
	for _, s := range []string{"worker 1", "worker 2", "worker 3"} {
		jobTopic.Subscribe(s)
	}
	jobTopic.Publish(map[string]int{"photo": 1})
	resized := 0
	for _, s := range jobTopic.keys {
		resized += len(jobTopic.subs[s])
	}
	fmt.Printf("trap 1: resize photo 1 sent to a topic, resized %d times\n", resized)

	// Trap 2: worker 1 takes the deposit but is slow, worker 2 finishes the withdrawal first.
	balance := 0
	if balance >= 80 {
		balance -= 80
	} else {
		fmt.Println("trap 2: withdraw 80 runs first, balance 0, refused")
	}
	balance += 100
	fmt.Printf("trap 2: then deposit 100, balance %d, the withdrawal was lost\n", balance)

	// Fix: both messages of account 7 go to the same worker, in order.
	w1, w2 := workerFor("account 7", 3), workerFor("account 7", 3)
	balance = 0
	balance += 100
	balance -= 80
	fmt.Printf("fix: deposit to worker %d, withdraw to worker %d, balance %d\n", w1+1, w2+1, balance)
}
