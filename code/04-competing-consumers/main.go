// Competing consumers: many workers on one queue, each job to one worker.
//
// The broker is simulated in ticks so the numbers are exact. A job handed
// out stays "unacked" until its worker acks it. If the worker dies first,
// the broker puts the job back, as RabbitMQ does when a connection drops.
// The second half shows message groups (SQS FIFO MessageGroupId): one
// in-flight message per group, so order holds per key.
package main

import "fmt"

type Broker struct {
	ready   []int       // jobs waiting
	unacked map[int]int // job -> worker holding it
}

// Deliver gives the head job to a worker and remembers who holds it.
func (b *Broker) Deliver(worker int) (int, bool) {
	if len(b.ready) == 0 {
		return 0, false
	}
	job := b.ready[0]
	b.ready = b.ready[1:]
	b.unacked[job] = worker
	return job, true
}

func (b *Broker) Ack(job int) { delete(b.unacked, job) }

// WorkerDied puts every job that worker held back at the head of the queue.
func (b *Broker) WorkerDied(worker int) []int {
	var back []int
	for job, w := range b.unacked {
		if w == worker {
			delete(b.unacked, job)
			b.ready = append([]int{job}, b.ready...)
			back = append(back, job)
		}
	}
	return back
}

func newBroker(n int) *Broker {
	b := &Broker{unacked: map[int]int{}}
	for j := 1; j <= n; j++ {
		b.ready = append(b.ready, j)
	}
	return b
}

// drain runs workers that each take one job per tick, limited by how many
// writes the shared image store accepts per tick.
func drain(workers, storeLimit int) int {
	b := newBroker(30)
	ticks := 0
	for len(b.ready) > 0 {
		ticks++
		for w := 1; w <= workers && w <= storeLimit; w++ {
			if job, ok := b.Deliver(w); ok {
				b.Ack(job)
			}
		}
	}
	return ticks
}

type Pay struct {
	Group  string
	Amount int
}

func main() {
	fmt.Println("== Tab 1: scale ==")
	fmt.Printf("step 1: 1 worker, 30 jobs, one job per tick: %d ticks\n", drain(1, 100))
	fmt.Printf("step 3: 3 workers: %d ticks\n", drain(3, 100))

	b := newBroker(30)
	for j := 1; j < 20; j++ { // jobs 1..19 done
		job, _ := b.Deliver(1)
		b.Ack(job)
	}
	job, _ := b.Deliver(2) // worker 2 takes job 20 and dies before the ack
	back := b.WorkerDied(2)
	again, _ := b.Deliver(3)
	fmt.Printf("step 4: worker 2 died holding job %d, put back %v, worker 3 gets job %d\n", job, back, again)

	fmt.Printf("step 5: the store takes 4 writes per tick: 4 workers %d ticks, 10 workers %d ticks\n",
		drain(4, 4), drain(10, 4))

	fmt.Println("== Tab 2: order ==")
	// Plain competing consumers: the withdrawal runs first.
	bal := 0
	if bal < 80 {
		fmt.Println("step 7: withdraw 80 runs first, balance 0, refused")
	}
	// Message groups: one in-flight message per group.
	queue := []Pay{{"account 7", 100}, {"account 9", 5}, {"account 7", -80}, {"account 12", 7}}
	balances := map[string]int{}
	tick := 0
	for len(queue) > 0 {
		tick++
		busy := map[string]bool{}
		var rest []Pay
		for _, p := range queue {
			if busy[p.Group] { // its group already has a message in flight
				rest = append(rest, p)
				continue
			}
			busy[p.Group] = true
			balances[p.Group] += p.Amount
		}
		queue = rest
	}
	fmt.Printf("step 9: groups: account 7 balance %d after %d ticks, 9 and 12 ran beside it\n", balances["account 7"], tick)

	// One busy key: 100 payments, 50 from one market maker, 10 workers.
	other := 50
	ticks := 50 // the market maker's group runs one at a time
	if other/10 > ticks {
		ticks = other / 10
	}
	fmt.Printf("step 10: 100 payments, 50 in one group, 10 workers: %d ticks (without groups 10)\n", ticks)
}
