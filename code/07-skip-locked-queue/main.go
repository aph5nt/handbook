// SKIP LOCKED queue: three workers take price fetch jobs from one jobs table.
// Plain FOR UPDATE makes them wait in line. SKIP LOCKED lets each take a free row.
// The last part claims in a short transaction and runs a reaper.
// Run: go run .
package main

import "fmt"

// Job is one row of the jobs table.
type Job struct {
	id           int
	status       string // ready, running, done
	claimedUntil int    // minute on the shared clock
}

// Queue is the jobs table plus the row locks open transactions hold.
type Queue struct {
	jobs  []*Job
	locks map[int]string // job id -> transaction holding its row lock
}

func NewQueue(n int) *Queue {
	q := &Queue{locks: map[int]string{}}
	for id := 1; id <= n; id++ {
		q.jobs = append(q.jobs, &Job{id: id, status: "ready"})
	}
	return q
}

// Pick is SELECT id FROM jobs WHERE status = 'ready' ORDER BY id LIMIT 1
// FOR UPDATE, with SKIP LOCKED when skip is true.
// It returns the job it locked, or the job it must wait for.
func (q *Queue) Pick(tx string, skip bool) (id, waitsFor int) {
	for _, j := range q.jobs {
		if j.status != "ready" {
			continue
		}
		if h, locked := q.locks[j.id]; locked && h != tx {
			if skip {
				continue // SKIP LOCKED: a locked row is treated as not there
			}
			return 0, j.id // plain FOR UPDATE: wait for this row
		}
		q.locks[j.id] = tx
		return j.id, 0
	}
	return 0, 0
}

// Commit writes the new status on every row tx locked, then frees the locks.
func (q *Queue) Commit(tx, status string) {
	for _, j := range q.jobs {
		if q.locks[j.id] == tx {
			j.status = status
			delete(q.locks, j.id)
		}
	}
}

// Rollback frees the locks of tx and keeps the rows as they were.
func (q *Queue) Rollback(tx string) {
	for id, h := range q.locks {
		if h == tx {
			delete(q.locks, id)
		}
	}
}

// Ready counts rows with status ready.
func (q *Queue) Ready() int {
	n := 0
	for _, j := range q.jobs {
		if j.status == "ready" {
			n++
		}
	}
	return n
}

// drain runs rounds of three workers until no job is ready.
// It returns how many jobs ran in each round.
func drain(q *Queue, skip bool) []int {
	rounds := []int{}
	for q.Ready() > 0 {
		ran := 0
		for w := 1; w <= 3; w++ {
			if id, _ := q.Pick(fmt.Sprintf("worker %d", w), skip); id != 0 {
				ran++
			}
		}
		for w := 1; w <= 3; w++ {
			q.Commit(fmt.Sprintf("worker %d", w), "done")
		}
		rounds = append(rounds, ran)
	}
	return rounds
}

func plain() {
	fmt.Println("== plain FOR UPDATE, five jobs, three workers")
	q := NewQueue(5)
	id, _ := q.Pick("worker 1", false)
	fmt.Printf("worker 1 locks job %d\n", id)
	_, w2 := q.Pick("worker 2", false)
	_, w3 := q.Pick("worker 3", false)
	fmt.Printf("worker 2 waits for job %d, worker 3 waits for job %d, jobs 2 to 5 free\n", w2, w3)
	q.Commit("worker 1", "done")
	fmt.Printf("jobs per round for a fresh queue: %v\n", drain(NewQueue(5), false))
}

func skipLocked() {
	fmt.Println("== SKIP LOCKED, five jobs, three workers")
	q := NewQueue(5)
	for w := 1; w <= 3; w++ {
		id, wait := q.Pick(fmt.Sprintf("worker %d", w), true)
		fmt.Printf("worker %d locks job %d, waits for %d\n", w, id, wait)
	}
	for w := 1; w <= 3; w++ {
		q.Commit(fmt.Sprintf("worker %d", w), "done")
	}
	fmt.Printf("after the commits: jobs 1 to 3 done, ready %d\n", q.Ready())
	a, _ := q.Pick("worker 1", true)
	b, _ := q.Pick("worker 2", true)
	fmt.Printf("next loop: worker 1 locks job %d, worker 2 locks job %d\n", a, b)
	q.Rollback("worker 2") // worker 2 crashes: its transaction rolls back
	_, held := q.locks[5]
	status := q.jobs[4].status
	fmt.Printf("worker 2 crashes: job 5 locked %v, status %s\n", held, status)
	c, _ := q.Pick("worker 3", true)
	q.Commit("worker 1", "done")
	q.Commit("worker 3", "done")
	fmt.Printf("worker 3 locks job %d and finishes it, ready %d\n", c, q.Ready())
	fmt.Printf("jobs per round for a fresh queue: %v\n", drain(NewQueue(5), true))
}

// Claim is the short transaction: pick with SKIP LOCKED, set status = 'running'
// and claimed_until = now + 15 minutes, commit at once.
func (q *Queue) Claim(tx string, now int) int {
	id, _ := q.Pick(tx, true)
	if id == 0 {
		return 0
	}
	q.jobs[id-1].claimedUntil = now + 15
	q.Commit(tx, "running") // the claim lives in the row, not in a lock
	return id
}

// Reap is UPDATE jobs SET status = 'ready' WHERE status = 'running'
// AND claimed_until < now(). It returns the jobs it put back.
func (q *Queue) Reap(now int) []int {
	back := []int{}
	for _, j := range q.jobs {
		if j.status == "running" && j.claimedUntil < now {
			j.status = "ready"
			back = append(back, j.id)
		}
	}
	return back
}

// keptDead counts the dead row versions cleanup cannot remove at minute now:
// any version that died after the oldest open transaction began.
func keptDead(oldestOpen, now, updatesPerMinute int) int {
	if oldestOpen < 0 {
		return 0 // no transaction open: cleanup removes every dead version
	}
	return (now - oldestOpen) * updatesPerMinute
}

func longJob() {
	fmt.Println("== a 10-minute export")
	q := NewQueue(3)
	for w := 1; w <= 3; w++ {
		q.Pick(fmt.Sprintf("worker %d", w), true) // BEGIN, lock, then work inside
	}
	fmt.Printf("claim and work in one transaction: %d row locks held for 10 minutes\n", len(q.locks))
	fmt.Printf("at minute 10 with 100 updates a minute elsewhere: dead versions kept %d\n", keptDead(0, 10, 100))

	q = NewQueue(3)
	id := q.Claim("worker 1", 0)
	fmt.Printf("short claim: job %d running, claimed until minute %d, row locks held %d\n", id, q.jobs[id-1].claimedUntil, len(q.locks))
	q.jobs[id-1].status = "done" // the export ran with no transaction open, then a short UPDATE
	fmt.Printf("work outside, then mark done: job %d %s, dead versions kept %d\n", id, q.jobs[id-1].status, keptDead(-1, 10, 100))

	lost := q.Claim("worker 2", 3) // worker 2 dies at minute 3 holding the claim
	fmt.Printf("worker 2 claims job %d at minute 3 and dies; reap at minute 10 puts back %v\n", lost, q.Reap(10))
	fmt.Printf("reap at minute 19 puts back %v\n", q.Reap(19))
}

func main() {
	plain()
	skipLocked()
	longJob()
}
