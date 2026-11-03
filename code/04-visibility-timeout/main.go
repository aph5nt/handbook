// Visibility timeout: a crash that heals itself, and a slow job that runs twice.
//
// A Go struct stands in for an SQS queue. Time is an integer in seconds, so
// nothing sleeps. Receive does not delete a job, it hides it until
// visibleAt. Delete is the acknowledgement. Deterministic.
package main

import "fmt"

type Job struct {
	ID        int
	visibleAt int
	receives  int
	deleted   bool
}

type Queue struct {
	timeout int
	jobs    []*Job
}

const maxReceive = 4 // the redrive policy: then to the DLQ

// Receive returns the first visible job and hides it for the timeout.
func (q *Queue) Receive(now int) *Job {
	for _, j := range q.jobs {
		if !j.deleted && j.visibleAt <= now {
			j.visibleAt = now + q.timeout
			j.receives++
			return j
		}
	}
	return nil
}

func (q *Queue) Delete(j *Job) { j.deleted = true }

// Extend is ChangeMessageVisibility: a heartbeat while still working.
func (q *Queue) Extend(j *Job, now int) { j.visibleAt = now + q.timeout }

// files is the image store: one file per name.
var files = map[string]bool{}

// put writes one output file. The name comes from the job id.
func put(path string) {
	files[path] = true
	fmt.Println("wrote", path)
}

// dlq holds the jobs the redrive policy moved out.
var dlq []*Job

// moveToDLQ takes a job out of the queue and parks it.
func moveToDLQ(j *Job) {
	j.deleted = true
	dlq = append(dlq, j)
	fmt.Printf("job %d moved to the DLQ, %d parked\n", j.ID, len(dlq))
}

func main() {
	fmt.Println("== Tab 1: hide, then delete ==")
	q := &Queue{timeout: 30, jobs: []*Job{{ID: 7}, {ID: 8}}}
	j := q.Receive(0)
	fmt.Printf("t=0 worker A receives job %d, hidden until t=%d\n", j.ID, j.visibleAt)
	q.Delete(j)
	fmt.Println("t=10 worker A deletes job 7")
	j = q.Receive(10)
	fmt.Printf("t=10 worker A receives job %d, hidden until t=%d, then crashes\n", j.ID, j.visibleAt)
	fmt.Printf("t=39 receive returns %v: job 8 still hidden\n", q.Receive(39))
	j = q.Receive(40)
	fmt.Printf("t=40 worker B receives job %d, receive count %d\n", j.ID, j.receives)
	q.Delete(j)
	poison := &Queue{timeout: 30, jobs: []*Job{{ID: 13}}}
	t := 0
	for {
		p := poison.Receive(t)
		if p.receives >= maxReceive {
			fmt.Printf("t=%d poison job %d: receive count %d\n", t, p.ID, p.receives)
			moveToDLQ(p)
			break
		}
		t += poison.timeout // the worker crashes every time
	}

	fmt.Println("== Tab 2: timeout too short ==")
	s := &Queue{timeout: 30, jobs: []*Job{{ID: 9}}}
	j = s.Receive(0)
	fmt.Printf("t=0 worker A receives job 9, it takes 45 s, hidden until t=%d\n", j.visibleAt)
	jb := s.Receive(30)
	fmt.Printf("t=30 job 9 visible again: worker B receives it, receive count %d\n", jb.receives)
	s.Delete(j)
	fmt.Println("t=45 worker A deletes job 9")
	put(fmt.Sprintf("resized/%d.png", j.ID))  // worker A at t=45
	put(fmt.Sprintf("resized/%d.png", jb.ID)) // a repeat overwrites the same file
	fmt.Printf("t=75 worker B finishes and writes the file again: job 9 ran 2 times, %d file\n", len(files))

	f := &Queue{timeout: 30, jobs: []*Job{{ID: 9}}}
	j = f.Receive(0)
	for _, beat := range []int{20, 40} {
		f.Extend(j, beat) // heartbeat: still working
		fmt.Printf("t=%d extend: hidden until t=%d\n", beat, j.visibleAt)
	}
	fmt.Printf("t=45 receive returns %v, worker A deletes job 9: ran once\n", f.Receive(45))
	f.Delete(j)
}
