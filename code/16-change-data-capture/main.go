// Change data capture versus polling: orders 41, 42 and 43 in Postgres,
// copied to StarRocks. CDC reads the change log, polling reads updated_at.
package main

import (
	"fmt"
	"sort"
)

type Row struct {
	ID        int
	Status    string
	UpdatedAt int // minute of the last update, if the writer set it
}

// Change is one write-ahead log record.
type Change struct {
	LSN    int // log position
	Op     string
	Before *Row
	After  *Row
}

type Postgres struct {
	rows map[int]Row
	wal  []Change
}

func (p *Postgres) write(op string, id int, status string, minute int, setUpdatedAt bool) {
	old, had := p.rows[id]
	var before, after *Row
	if had {
		b := old
		before = &b
	}
	if op == "delete" {
		delete(p.rows, id)
	} else {
		r := Row{ID: id, Status: status, UpdatedAt: old.UpdatedAt}
		if setUpdatedAt {
			r.UpdatedAt = minute
		}
		p.rows[id] = r
		after = &r
	}
	// the change is logged before it is applied, every time
	p.wal = append(p.wal, Change{LSN: len(p.wal) + 1, Op: op, Before: before, After: after})
}

// Debezium reads the log from its saved position and publishes events.
type Debezium struct{ saved int }

func (d *Debezium) Poll(p *Postgres) []Change {
	var out []Change
	for _, c := range p.wal {
		if c.LSN > d.saved { // only records after the saved position
			out = append(out, c)
		}
	}
	if len(out) > 0 {
		d.saved = out[len(out)-1].LSN // save the new position: the checkpoint
	}
	return out
}

// Apply upserts by key, and removes the row on a delete event.
func Apply(wh map[int]Row, events []Change) {
	for _, e := range events {
		switch e.Op {
		case "delete":
			delete(wh, e.Before.ID)
		default:
			wh[e.After.ID] = *e.After // same key twice gives one row
		}
	}
}

func show(name string, m map[int]Row) {
	ids := []int{}
	for id := range m {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	fmt.Print(name, ":")
	for _, id := range ids {
		fmt.Printf(" %d=%s", id, m[id].Status)
	}
	fmt.Println()
}

func main() {
	pg := &Postgres{rows: map[int]Row{}}
	for _, id := range []int{41, 42, 43} {
		pg.rows[id] = Row{ID: id, Status: "new", UpdatedAt: 0}
	}
	// CDC: snapshot, then the log
	wh := map[int]Row{}
	for id, r := range pg.rows {
		wh[id] = r
	}
	d := &Debezium{}
	show("snapshot", wh)
	pg.write("update", 42, "paid", 1, true)
	ev := d.Poll(pg)
	fmt.Printf("log read from position 0: %d event, order 42 before %s after %s, saved position %d\n", len(ev), ev[0].Before.Status, ev[0].After.Status, d.saved)
	Apply(wh, ev)
	Apply(wh, ev) // delivered twice: still one row
	show("after upsert (applied twice)", wh)
	pg.write("delete", 43, "", 2, true)
	Apply(wh, d.Poll(pg))
	show("after delete event", wh)
	// the failure: the reader stops, Postgres keeps every unread record
	for m := 0; m < 4; m++ {
		pg.write("update", 41, "new", 3+m, true)
	}
	held := len(pg.wal) - d.saved // alert on this: the replication slot lag
	fmt.Printf("reader stopped at position %d, log records held for it: %d\n", d.saved, held)

	// Polling: SELECT * WHERE updated_at > last
	pg2 := &Postgres{rows: map[int]Row{}}
	wh2 := map[int]Row{}
	for _, id := range []int{41, 42, 43} {
		pg2.rows[id] = Row{ID: id, Status: "new"}
		wh2[id] = pg2.rows[id]
	}
	last := 0
	poll := func() {
		for id, r := range pg2.rows {
			if r.UpdatedAt > last { // only rows whose updated_at moved
				wh2[id] = r
			}
		}
		last++
	}
	pg2.write("update", 42, "paid", 1, true)
	poll()
	pg2.write("delete", 43, "", 2, true)
	poll()
	pg2.write("update", 41, "cancelled", 3, false) // repair script forgot updated_at
	poll()
	show("postgres", pg2.rows)
	show("polled copy", wh2)
	wrong := 0
	for id, r := range wh2 {
		if src, ok := pg2.rows[id]; !ok || src.Status != r.Status {
			wrong++
		}
	}
	fmt.Printf("polling: %d of %d warehouse rows wrong, no error reported\n", wrong, len(wh2))
}
