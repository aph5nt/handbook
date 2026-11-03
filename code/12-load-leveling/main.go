// Command loadleveling replays the lesson: agents put graph jobs on a queue and
// workers pull them at their own pace. A burst is levelled into a flat load. A lasting
// rise above the drain rate makes the queue grow, the jobs go stale, and only more
// workers (scaled on lag, the age of the oldest job) bring it back.
// Time moves in whole minutes, so every run prints the same numbers.
package main

import "fmt"

// batch is the jobs that arrived in one minute and are still waiting.
type batch struct{ minute, jobs int }

// Queue is first in, first out, kept as batches per arrival minute.
type Queue struct {
	batches []batch
	Limit   int // jobs that may wait; 0 means no limit
}

// Put adds the jobs that arrived this minute and returns how many it refused.
func (q *Queue) Put(minute, jobs int) (refused int) {
	if q.Limit > 0 {
		if room := q.Limit - q.Depth(); jobs > room {
			refused = jobs - room
			jobs = room
		}
	}
	if jobs > 0 {
		q.batches = append(q.batches, batch{minute, jobs})
	}
	return refused
}

// Take lets the workers pull up to n jobs, oldest first.
func (q *Queue) Take(n int) {
	for n > 0 && len(q.batches) > 0 {
		b := &q.batches[0]
		k := min(n, b.jobs)
		b.jobs -= k
		n -= k
		if b.jobs == 0 {
			q.batches = q.batches[1:]
		}
	}
}

// Depth is how many jobs wait.
func (q *Queue) Depth() int {
	d := 0
	for _, b := range q.batches {
		d += b.jobs
	}
	return d
}

// Lag is the age in minutes of the oldest waiting job: the number to scale on.
func (q *Queue) Lag(now int) int {
	if len(q.batches) == 0 {
		return 0
	}
	return now - q.batches[0].minute
}

// Minute runs one minute: the jobs arrive, then the workers take what they can.
func (q *Queue) Minute(now, arrivals, drain int) {
	q.Put(now, arrivals)
	q.Take(drain)
}

func main() {
	const workers = 100 // jobs a minute the workers run
	q := &Queue{}
	now := 0
	run := func(minutes, arrivals, drain int) {
		for k := 0; k < minutes; k++ {
			now++
			q.Minute(now, arrivals, drain)
		}
	}

	run(3, 40, workers)
	fmt.Printf("normal morning, 40 a minute: %d waiting\n", q.Depth())
	for k := 0; k < 3; k++ {
		run(1, 300, workers)
		fmt.Printf("burst minute %d, 300 a minute: %d waiting, graph still runs %d a minute\n", k+1, q.Depth(), workers)
	}
	drained := 0
	for q.Depth() > 0 {
		run(1, 40, workers)
		drained++
	}
	fmt.Printf("back to 40 a minute: empty again after %d minutes\n", drained)
	fmt.Printf("the last job of the burst waited %d minutes (600 ahead of it / %d a minute)\n", 600/workers, workers)

	// Arrivals rise to 120 a minute and stay there.
	q, now = &Queue{}, 0
	run(2, 40, workers)
	run(14, 120, workers)
	fmt.Printf("\n120 a minute for 14 minutes: %d waiting, oldest %d minutes old\n", q.Depth(), q.Lag(now))
	saved := &Queue{batches: append([]batch(nil), q.batches...)}
	savedNow := now
	timeout, retention := 0, 0
	for retention == 0 {
		run(1, 120, workers)
		if timeout == 0 && q.Lag(now) >= 10 {
			timeout = now
			fmt.Printf("minute %d: %d waiting, oldest %d minutes old: past the agents' 10 minute timeout\n", now, q.Depth(), q.Lag(now))
		}
		if q.Lag(now) >= 60 {
			retention = now
			fmt.Printf("minute %d (%.1f hours): %d waiting, oldest %d minutes old: the 1 hour retention starts deleting jobs\n",
				now, float64(now)/60, q.Depth(), q.Lag(now))
		}
	}

	// The fix: scale workers to 150 a minute when lag passes the target.
	q, now = saved, savedNow
	start := q.Depth()
	m := 0
	for q.Depth() > 0 {
		run(1, 120, 150)
		m++
	}
	fmt.Printf("\nworkers at 150 a minute: %d waiting drains in %d minutes (30 a minute)\n", start, m)

	// And bound the queue: at the limit a put is refused with an error.
	b := &Queue{Limit: 200}
	refused := 0
	for t := 1; t <= 14; t++ {
		refused += b.Put(t, 120)
		b.Take(100)
	}
	fmt.Printf("limit 200 at 120 a minute for 14 minutes: %d waiting, %d refused with an error\n", b.Depth(), refused)
}
