// An indexer that writes keyed rows and a checkpoint, with a crash injected
// between the two writes. Data first gives at-least-once delivery and an
// exactly-once effect. Checkpoint first loses a block. No keys doubles it.
package main

import (
	"errors"
	"fmt"
)

type Key struct{ Block, Log int } // the event itself names the row

type Store struct {
	rows       map[Key]string // keyed: a second write overwrites
	appended   []Key          // the no-key design: a second write appends
	checkpoint int
	alerts     []string
	sentAlerts map[Key]bool // dedup for side effects, by event id (a stored table in production)
}

var errCrash = errors.New("process killed")

// events in each block: two transfers, the second one is large
func eventsOf(block int) []Key { return []Key{{block, 0}, {block, 1}} }

// IndexBlock: data first, then the checkpoint. crash simulates a kill
// between the two writes.
func (s *Store) IndexBlock(b int, crash, dedup bool) error {
	for _, k := range eventsOf(b) {
		s.rows[k] = "transfer" // upsert by key
		if k.Log == 1 {
			if dedup && s.sentAlerts[k] {
				continue // already sent for this event id
			}
			s.sentAlerts[k] = true
			s.alerts = append(s.alerts, fmt.Sprintf("alert %d log %d", k.Block, k.Log))
		}
	}
	if crash {
		return errCrash
	}
	s.checkpoint = b // only after the rows are stored
	return nil
}

// IndexBlockCheckpointFirst is the trap: the mark moves before the data.
func (s *Store) IndexBlockCheckpointFirst(b int, crash bool) error {
	s.checkpoint = b
	if crash {
		return errCrash // the rows of b are never written
	}
	for _, k := range eventsOf(b) {
		s.rows[k] = "transfer"
	}
	return nil
}

// IndexBlockAppend is the other trap: rows with no event key.
func (s *Store) IndexBlockAppend(b int, crash bool) error {
	s.appended = append(s.appended, eventsOf(b)...)
	if crash {
		return errCrash
	}
	s.checkpoint = b
	return nil
}

func newStore() *Store {
	return &Store{rows: map[Key]string{}, checkpoint: 9505, sentAlerts: map[Key]bool{}}
}

func main() {
	s := newStore()
	_ = s.IndexBlock(9506, false, false)
	fmt.Printf("data first: block 9506 done, rows %d, checkpoint %d\n", len(s.rows), s.checkpoint)
	err := s.IndexBlock(9507, true, false)
	fmt.Printf("data first: block 9507 %v, rows %d, checkpoint %d\n", err, len(s.rows), s.checkpoint)
	// restart: resume after the checkpoint
	next := s.checkpoint + 1
	_ = s.IndexBlock(next, false, false)
	fmt.Printf("restart at %d: rows %d (not 6), checkpoint %d, alerts %d %v\n", next, len(s.rows), s.checkpoint, len(s.alerts), s.alerts)

	s = newStore()
	_ = s.IndexBlock(9506, false, true)
	_ = s.IndexBlock(9507, true, true)
	_ = s.IndexBlock(s.checkpoint+1, false, true)
	fmt.Printf("with alert dedup by event id: alerts %d %v\n", len(s.alerts), s.alerts)

	s = newStore()
	_ = s.IndexBlockCheckpointFirst(9506, false)
	err = s.IndexBlockCheckpointFirst(9507, true)
	fmt.Printf("checkpoint first: block 9507 %v, rows %d, checkpoint %d\n", err, len(s.rows), s.checkpoint)
	next = s.checkpoint + 1
	_ = s.IndexBlockCheckpointFirst(next, false)
	_, has := s.rows[Key{9507, 0}]
	fmt.Printf("restart at %d: rows %d, checkpoint %d, block 9507 present: %v\n", next, len(s.rows), s.checkpoint, has)

	s = newStore()
	_ = s.IndexBlockAppend(9506, false)
	_ = s.IndexBlockAppend(9507, true)
	_ = s.IndexBlockAppend(s.checkpoint+1, false)
	fmt.Printf("no keys: rows %d after the replay of 9507 (should be 4)\n", len(s.appended))
}
