// Write-ahead log and log-based CDC, against a poller, and the stuck slot.
//
// A slice stands in for the Postgres WAL and an int for the replication
// slot's confirmed position. Debezium reads from the slot and saves its
// position only after publishing. Disk numbers are this example's.
// Deterministic.
package main

import "fmt"

type Entry struct {
	LSN       int
	Op, Order string
	Status    string
}

type DB struct {
	wal  []Entry
	rows map[string]string
}

// Write logs the change first, then updates the table.
func (d *DB) Write(op, order, status string) {
	d.wal = append(d.wal, Entry{len(d.wal) + 1, op, order, status})
	if op == "delete" {
		delete(d.rows, order)
	} else {
		d.rows[order] = status
	}
}

// Debezium reads the WAL after its saved position.
type Debezium struct{ saved int }

func (z *Debezium) Stream(d *DB, crashBeforeSave bool) []string {
	var out []string
	pos := z.saved
	for pos < len(d.wal) {
		e := d.wal[pos]
		out = append(out, fmt.Sprintf("%s %s %s", e.Op, e.Order, e.Status))
		pos++
		if crashBeforeSave {
			return out // published, not saved: sent again after a restart
		}
		z.saved = pos
	}
	return out
}

func main() {
	fmt.Println("== Tab 1: the log ==")
	d := &DB{rows: map[string]string{}}
	d.Write("insert", "500", "placed")
	z := &Debezium{}
	fmt.Println("published:", z.Stream(d, false), "saved position", z.saved)
	d.Write("update", "500", "paid")
	fmt.Println("crash before save, published:", z.Stream(d, true), "saved position", z.saved)
	fmt.Println("restart, published again:", z.Stream(d, false), "saved position", z.saved)

	fmt.Println("== Tab 2: a poller ==")
	d = &DB{rows: map[string]string{}}
	d.Write("insert", "501", "placed")
	d.Write("insert", "502", "placed")
	seen := map[string]string{"501": d.rows["501"], "502": d.rows["502"]} // poll at 0 s
	start := len(d.wal)
	// inside one 5-second gap
	d.Write("update", "501", "paid")
	d.Write("update", "501", "cancelled")
	d.Write("delete", "502", "")
	// the poller only ever sees the current row
	fmt.Println("next poll sees 501:", d.rows["501"]) // cancelled: placed and paid are gone
	_, ok := d.rows["502"]                            // false: the delete is invisible
	fmt.Printf("next poll finds 502: %v (was %s): the delete is invisible\n", ok, seen["502"])
	z = &Debezium{saved: start}
	fmt.Println("the log sees all:", z.Stream(d, false))

	fmt.Println("== Tab 3: the stuck slot ==")
	const diskGB, walPerHour = 100, 2 // this example's numbers
	kept, hour := 0, 0
	for kept < diskGB {
		hour++
		kept += walPerHour // nothing is confirmed, so nothing is removed
	}
	fmt.Printf("connector stopped: WAL grows %d GB/h, a %d GB disk is full after %d hours: commits stop\n", walPerHour, diskGB, hour)
	const keepGB = 50                // max_slot_wal_keep_size
	dropAfter := keepGB / walPerHour // 25 hours: the database survives
	fmt.Printf("max_slot_wal_keep_size %d GB: slot dropped after %d hours, the database keeps running, the connector re-snapshots\n", keepGB, dropAfter)
}
