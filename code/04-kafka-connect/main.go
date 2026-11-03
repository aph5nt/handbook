// Kafka Connect: a source and a sink from configuration, stored positions,
// a worker dying, and the bad record that stops a sink.
//
// Go structs stand in for the Connect cluster. Config is parsed from the
// same JSON a team would POST to the REST API. Tasks store their position in
// a map that plays the connect-offsets topic, so a restart resumes.
// Deterministic.
package main

import (
	"encoding/json"
	"fmt"
	"strconv"
)

const sinkConfig = `{"name":"orders-s3","tasks.max":"3","errors.tolerance":"none"}`

type Task struct {
	ID     int
	Worker int
	State  string // RUNNING or FAILED
}

type Connector struct {
	Name, Tolerance string
	Tasks           []*Task
	offsets         map[int]int // partition -> next offset: plays connect-offsets
	DLQ             []string
	S3              []string
}

func start(cfg string, workers int) *Connector {
	var c map[string]string
	if err := json.Unmarshal([]byte(cfg), &c); err != nil {
		panic(err)
	}
	n, _ := strconv.Atoi(c["tasks.max"])
	k := &Connector{Name: c["name"], Tolerance: c["errors.tolerance"], offsets: map[int]int{}}
	for i := 0; i < n; i++ {
		k.Tasks = append(k.Tasks, &Task{i + 1, i%workers + 1, "RUNNING"})
	}
	return k
}

// rebalance moves tasks off a dead worker. Their offsets stay in Kafka.
func (k *Connector) rebalance(dead int, alive []int) {
	next := 0
	for _, t := range k.Tasks {
		if t.Worker == dead {
			t.Worker = alive[next%len(alive)]
			next++
		}
	}
}

// put writes one record, the way a sink task does.
func (k *Connector) put(t *Task, partition int, rec string) {
	if t.State != "RUNNING" {
		return // a failed task does nothing, and nothing restarts it
	}
	if rec == "BAD" {
		if k.Tolerance != "all" {
			t.State = "FAILED"
			return
		}
		k.DLQ = append(k.DLQ, rec) // errors.tolerance=all: park it, go on
	} else {
		k.S3 = append(k.S3, rec)
	}
	k.offsets[partition]++
}

// transform is a single message transform: small, stateless, on the way.
func transform(row map[string]string) map[string]string {
	delete(row, "internal_note")
	row["order_id"] = row["id"]
	delete(row, "id")
	return row
}

// alert pages the on-call engineer about a task that is not RUNNING.
func alert(t *Task) {
	fmt.Printf("alert: task %d is %s\n", t.ID, t.State)
}

func main() {
	fmt.Println("== Tab 1: source and sink ==")
	walChanges := []map[string]string{{"id": "500", "internal_note": "x", "status": "placed"}}
	var topic []map[string]string
	// source task loop: WAL change -> transform -> Kafka
	for _, row := range walChanges {
		topic = append(topic, transform(row))
	}
	fmt.Println("source wrote to Kafka:", topic)
	k := start(sinkConfig, 3)
	fmt.Printf("connector %s from JSON: %d tasks\n", k.Name, len(k.Tasks))
	for i := 0; i < 3; i++ {
		k.put(k.Tasks[0], 0, fmt.Sprintf("order %d", 500+i))
	}
	fmt.Printf("task 1 wrote %d records, stored offset %d\n", len(k.S3), k.offsets[0])

	fmt.Println("== Tab 2: workers and failures ==")
	k.rebalance(2, []int{1, 3})
	fmt.Printf("worker 2 died: task 2 moved to worker %d, task 1 resumes at offset %d\n", k.Tasks[1].Worker, k.offsets[0])
	k.put(k.Tasks[0], 0, "BAD")
	before := len(k.S3)
	k.put(k.Tasks[0], 0, "order 503")
	k.put(k.Tasks[0], 0, "order 504")
	fmt.Printf("bad record: task 1 is %s, offset stays %d, %d more written, worker pods healthy\n", k.Tasks[0].State, k.offsets[0], len(k.S3)-before)

	cfg := `{"name":"orders-s3","tasks.max":"3","errors.tolerance":"all"}`
	f := start(cfg, 3)
	for _, r := range []string{"order 500", "BAD", "order 501"} {
		f.put(f.Tasks[0], 0, r)
	}
	fmt.Printf("errors.tolerance=all: task %s, S3 %d, dead-letter topic %d\n", f.Tasks[0].State, len(f.S3), len(f.DLQ))
	for _, t := range k.Tasks {
		if t.State != "RUNNING" {
			alert(t)
		}
	}
}
