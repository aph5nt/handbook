// Outbox: the wallet row and the event are one transaction in one database.
// A relay publishes afterwards, so delivery is at least once.
package main

import (
	"errors"
	"fmt"
)

// Row is one outbox row.
type Row struct {
	Event string
	Sent  bool
}

// DB is Postgres: the wallet row and the outbox table.
type DB struct {
	Balance int
	Outbox  []*Row
}

// Kafka is the broker's topic.
type Kafka struct{ Messages []string }

// TopUp is one transaction: both rows exist, or neither does.
func TopUp(db *DB, amount int) {
	db.Balance += amount // the wallet row
	db.Outbox = append(db.Outbox, &Row{Event: "WalletToppedUp"})
	// COMMIT: both rows exist, or neither does
}

// Relay publishes every unsent row. crashBeforeMark stops it after the publish.
func Relay(db *DB, k *Kafka, crashBeforeMark bool) {
	for _, r := range db.Outbox {
		if r.Sent {
			continue
		}
		k.Messages = append(k.Messages, r.Event)
		if crashBeforeMark {
			return // row still unsent: the restart sends it again
		}
		r.Sent = true
	}
}

// CommitThenPublish is the failure: a crash between the two loses the event.
func CommitThenPublish(db *DB, k *Kafka, amount int, crash bool) {
	db.Balance += amount // committed
	if crash {
		return // nothing written down, nothing can resend it
	}
	k.Messages = append(k.Messages, "WalletToppedUp")
}

// PublishThenCommit fails the other way: a ghost event.
func PublishThenCommit(db *DB, k *Kafka, amount int, commitFails bool) error {
	k.Messages = append(k.Messages, "WalletToppedUp")
	if commitFails {
		return errors.New("commit timed out: rolled back")
	}
	db.Balance += amount
	return nil
}

func main() {
	db, k := &DB{}, &Kafka{}
	TopUp(db, 20)
	fmt.Println("after commit: balance", db.Balance, "outbox rows", len(db.Outbox), "messages", len(k.Messages))
	Relay(db, k, true)
	fmt.Println("crash before mark: messages", len(k.Messages), "row sent", db.Outbox[0].Sent)
	Relay(db, k, false)
	fmt.Println("restart: messages", len(k.Messages), "row sent", db.Outbox[0].Sent)
	Relay(db, k, false)
	fmt.Println("next poll skips it: messages", len(k.Messages))
	d2, k2 := &DB{}, &Kafka{}
	CommitThenPublish(d2, k2, 20, true)
	fmt.Println("commit then publish, crash: balance", d2.Balance, "messages", len(k2.Messages))
	d3, k3 := &DB{}, &Kafka{}
	err := PublishThenCommit(d3, k3, 20, true)
	fmt.Println("publish then commit:", err, "- balance", d3.Balance, "messages", len(k3.Messages))
}
