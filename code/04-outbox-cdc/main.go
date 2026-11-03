// Outbox and Change Data Capture: why one transaction beats a dual write.
//
// Go values stand in for Postgres and Kafka. DB.Commit applies a whole
// transaction at once, and each committed change is appended to the
// write-ahead log (WAL). The relay plays Debezium: it reads the WAL from its
// saved position and publishes outbox rows. Crashes are explicit. Deterministic.
package main

import "fmt"

type Change struct {
	Table, Key string
}

type DB struct {
	orders map[string]bool
	wal    []Change
}

// Commit applies all changes or none, and logs them in commit order.
func (d *DB) Commit(changes ...Change) {
	for _, c := range changes {
		if c.Table == "orders" {
			d.orders[c.Key] = true
		}
		d.wal = append(d.wal, c)
	}
}

type Relay struct{ pos int } // saved WAL position

// Poll publishes every outbox row after its position.
func (r *Relay) Poll(d *DB, kafka *[]string, crashBeforeSave bool) {
	for r.pos < len(d.wal) {
		c := d.wal[r.pos]
		if c.Table == "outbox" {
			*kafka = append(*kafka, "OrderPlaced "+c.Key)
		}
		if crashBeforeSave {
			return // published, position not saved: published again later
		}
		r.pos++
	}
}

func main() {
	fmt.Println("== Dual write: commit, then publish ==")
	d := &DB{orders: map[string]bool{}}
	var kafka []string
	d.Commit(Change{"orders", "501"})
	crashed := true // the pod dies here, before the publish line
	if !crashed {
		kafka = append(kafka, "OrderPlaced 501")
	}
	fmt.Printf("orders has 501: %v, events in Kafka: %d: shipping never hears of it\n", d.orders["501"], len(kafka))

	fmt.Println("== Outbox: one transaction, CDC publishes ==")
	d = &DB{orders: map[string]bool{}}
	kafka = nil
	d.Commit(Change{"orders", "502"}, Change{"outbox", "502"})
	fmt.Printf("committed order and outbox row, the app crashes now, events in Kafka: %d\n", len(kafka))
	r := &Relay{}
	r.Poll(d, &kafka, false)
	fmt.Printf("relay reads the WAL after the crash: events in Kafka: %v\n", kafka)

	fmt.Println("== Relay crash: at-least-once ==")
	d = &DB{orders: map[string]bool{}}
	kafka = nil
	d.Commit(Change{"outbox", "503"})
	r = &Relay{}
	r.Poll(d, &kafka, true)
	r.Poll(d, &kafka, false)
	fmt.Printf("relay crashed before saving its position: events %v\n", kafka)
	inbox := map[string]bool{}
	handled := 0
	for _, e := range kafka {
		if !inbox[e] { // the consumer's inbox drops the repeat
			inbox[e] = true
			handled++
		}
	}
	fmt.Printf("consumer inbox: handled %d of %d\n", handled, len(kafka))
}
