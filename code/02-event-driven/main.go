// Replays the event-driven lesson. Tab 1: the indexer writes rows and a
// checkpoint; the graph sync and a later validator each read from their own
// position; a crash makes the validator read 100 blocks twice. Tab 2: a
// decoder bug, the wrong in-place fix, and the reset that rewinds everyone.
package main

import "fmt"

// Log is the indexer's rows plus its checkpoint.
type Log struct {
	rows       map[int]int // block -> decoded value
	checkpoint int         // every block up to here is written
}

// Indexer is the producer. It never calls a reader.
type Indexer struct {
	bugFrom, bugTo int // blocks the decoder bug touches, 0 = no bug
}

func correct(block int) int { return block * 7 } // the right decoded value

func (ix *Indexer) decode(block int) int {
	if block >= ix.bugFrom && block <= ix.bugTo {
		return -1 // the bug: a wrong row
	}
	return correct(block)
}

// write appends rows, then moves the checkpoint. It knows no consumer.
func (ix *Indexer) write(log *Log, to int) {
	for b := log.checkpoint + 1; b <= to; b++ {
		log.rows[b] = ix.decode(b)
	}
	log.checkpoint = to // then the checkpoint: all up to here is written
}

// Reader is a consumer with its own position.
type Reader struct {
	name     string
	position int
	store    map[int]int // keyed by block, so a repeat overwrites: idempotent
	reads    int         // deliveries, repeats included
}

// read handles blocks after its own position, up to stop, then saves the
// position. crash=true stops before the save.
func (r *Reader) read(log *Log, stop int, crash bool) {
	for b := r.position + 1; b <= stop && b <= log.checkpoint; b++ {
		r.store[b] = log.rows[b] // same block, same value: a repeat is harmless
		r.reads++
	}
	if !crash { // a crash skips the save
		r.position = min(stop, log.checkpoint) // save my own position
	}
}

// wrongBlocks counts blocks where the reader holds a wrong value.
func (r *Reader) wrongBlocks() int {
	n := 0
	for b, v := range r.store {
		if v != correct(b) {
			n++
		}
	}
	return n
}

// reset: everyone is stopped. Rows, checkpoint and every position go back to
// `to` together, and each reader drops what it built after `to`.
func reset(log *Log, to int, readers ...*Reader) (deleted int) {
	for b := range log.rows {
		if b > to {
			delete(log.rows, b)
			deleted++
		}
	}
	log.checkpoint = to
	for _, r := range readers {
		for b := range r.store {
			if b > to {
				delete(r.store, b)
			}
		}
		r.position = to
	}
	return
}

func newReader(name string, at int) *Reader {
	return &Reader{name: name, position: at, store: map[int]int{}}
}

func main() {
	fmt.Println("tab 1: add a reader")
	log := &Log{rows: map[int]int{}}
	ix := &Indexer{}
	ix.write(log, 9400)
	gs := newReader("graph sync", 9400)
	fmt.Printf("  indexer checkpoint %d, graph sync position %d\n", log.checkpoint, gs.position)
	ix.write(log, 9506)
	fmt.Printf("  indexer wrote %d blocks, checkpoint %d, calls to readers 0\n", 9506-9400, log.checkpoint)
	gs.read(log, 9506, false)
	fmt.Printf("  graph sync read %d blocks, position %d\n", gs.reads, gs.position)
	va := newReader("validator", 9300)
	fmt.Printf("  validator joins at %d, indexer changed: no\n", va.position)
	va.read(log, 9400, true)
	fmt.Printf("  validator checked %d blocks, crashed, position still %d\n", va.reads, va.position)
	va.read(log, 9506, false)
	fmt.Printf("  after restart: deliveries %d, distinct blocks %d, repeats %d, wrong %d\n", va.reads, len(va.store), va.reads-len(va.store), va.wrongBlocks())
	fmt.Printf("  indexer %d, graph sync %d, validator %d\n", log.checkpoint, gs.position, va.position)

	fmt.Println("tab 2: bad rows and the reset")
	log = &Log{rows: map[int]int{}}
	ix = &Indexer{}
	ix.write(log, 9499)
	gs, va = newReader("graph sync", 9300), newReader("validator", 9300)
	ix.bugFrom, ix.bugTo = 9500, 9506
	ix.write(log, 9506)
	gs.read(log, 9506, false)
	va.read(log, 9506, false)
	fmt.Printf("  bad rows 9500 to 9506, checkpoint %d, graph wrong blocks %d, validator wrong blocks %d\n", log.checkpoint, gs.wrongBlocks(), va.wrongBlocks())

	// wrong fix: rewrite the rows in place while everyone keeps going
	ix.bugFrom, ix.bugTo = 0, 0
	ix.write(log, 9600)
	gs.read(log, 9600, false)
	va.read(log, 9600, false)
	for b := 9500; b <= 9506; b++ {
		log.rows[b] = correct(b)
	}
	gs.read(log, 9600, false) // reads only after 9600: nothing
	fmt.Printf("  in-place fix: rows right, graph sync at %d, graph wrong blocks %d\n", gs.position, gs.wrongBlocks())

	// right fix: stop all, rewind all to 9499, start together
	n := reset(log, 9499, gs, va)
	fmt.Printf("  reset: rows deleted %d, checkpoint %d, graph sync %d, validator %d\n", n, log.checkpoint, gs.position, va.position)
	ix.write(log, 9600)
	gs.read(log, 9600, false)
	va.read(log, 9600, false)
	fmt.Printf("  restart: indexer %d, graph sync %d, validator %d, graph wrong blocks %d\n", log.checkpoint, gs.position, va.position, gs.wrongBlocks())
}
