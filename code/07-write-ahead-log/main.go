// Write-ahead log (WAL): a payment sets wallet 7 from 10 to 2, the power
// fails, and the database restarts. With the log flushed at commit, replay
// brings the payment back. With fsync turned off, the payment is gone.
// Run: go run .
package main

import "fmt"

// Record is one WAL record: "wallet 7 set to 2", and whether it committed.
type Record struct {
	lsn     int
	wallet  int
	balance int
	commit  bool
}

// Disk is what survives a power cut: the WAL file and the table pages.
type Disk struct {
	wal        []Record
	pages      map[int]int
	checkpoint int // the last record already in the pages
}

// Engine is the running database: pages in memory, the log buffer, and the
// operating system's cache of file writes that are not on disk yet.
type Engine struct {
	disk    *Disk
	pages   map[int]int
	logBuf  []Record
	osCache []Record
	nextLSN int
	// fsync = off: never wait for the disk. synchronous_commit = off: answer
	// first, and let the WAL writer flush a moment later.
	fsync      bool
	syncCommit bool
}

func newDisk() *Disk {
	return &Disk{wal: []Record{{lsn: 41, wallet: 3, balance: 70, commit: true}}, pages: map[int]int{3: 70, 7: 10}, checkpoint: 41}
}

// Update changes the page in memory and writes a record into the log buffer.
// Nothing reaches the disk yet.
func (e *Engine) Update(wallet, balance int) Record {
	e.pages[wallet] = balance
	r := Record{lsn: e.nextLSN, wallet: wallet, balance: balance}
	e.nextLSN++
	e.logBuf = append(e.logBuf, r)
	return r
}

// Commit appends the log buffer to the WAL file. With fsync and
// synchronous_commit on, it waits until the disk holds the record. Only then
// does it answer "committed".
func (e *Engine) Commit() string {
	for _, r := range e.logBuf {
		r.commit = true
		e.osCache = append(e.osCache, r) // write(): the OS has it, the disk may not
	}
	e.logBuf = nil
	if e.fsync && e.syncCommit {
		e.Flush()
	}
	return "committed"
}

// Flush is fsync: the OS cache of the WAL file goes to the disk. With
// synchronous_commit = off, the WAL writer calls it a moment after commit.
func (e *Engine) Flush() {
	if !e.fsync {
		return // fsync = off: the write stays in the OS cache
	}
	e.disk.wal = append(e.disk.wal, e.osCache...)
	e.osCache = nil
}

// PowerCut wipes everything that is not on disk: memory and the OS cache.
func (e *Engine) PowerCut() *Disk {
	return e.disk
}

// Restart loads the pages from disk and replays the WAL after the
// checkpoint: committed records are redone, the rest are dropped.
func Restart(d *Disk) (e *Engine, replayed int) {
	e = &Engine{disk: d, pages: map[int]int{}, fsync: true, syncCommit: true}
	for id, b := range d.pages {
		e.pages[id] = b
	}
	for _, r := range d.wal {
		if r.lsn > d.checkpoint && r.commit {
			e.pages[r.wallet] = r.balance
			replayed++
		}
		e.nextLSN = r.lsn + 1
	}
	return e, replayed
}

// Checkpoint writes the changed pages to disk and records how far the log
// is applied. WAL before that point can be recycled.
func (e *Engine) Checkpoint() {
	for id, b := range e.pages {
		e.disk.pages[id] = b
	}
	e.disk.checkpoint = e.disk.wal[len(e.disk.wal)-1].lsn
}

func walOn() {
	fmt.Println("== fsync on: wallet 7, 10 to 2")
	e, _ := Restart(newDisk())
	fmt.Printf("start: memory %d, page on disk %d\n", e.pages[7], e.disk.pages[7])

	r := e.Update(7, 2)
	fmt.Printf("UPDATE: page in memory %d, log buffer record %d: wallet %d = %d\n", e.pages[7], r.lsn, r.wallet, r.balance)

	answer := e.Commit()
	last := e.disk.wal[len(e.disk.wal)-1]
	fmt.Printf("COMMIT: record %d flushed to the WAL, then %q; page on disk still %d\n", last.lsn, answer, e.disk.pages[7])

	d := e.PowerCut()
	fmt.Printf("power cut: memory wiped, page on disk %d, WAL ends at record %d\n", d.pages[7], d.wal[len(d.wal)-1].lsn)

	e, n := Restart(d)
	fmt.Printf("restart: replayed %d record after checkpoint %d, wallet 7 = %d\n", n, d.checkpoint, e.pages[7])

	e.Checkpoint()
	fmt.Printf("checkpoint: page on disk %d, log applied up to record %d\n", d.pages[7], d.checkpoint)
}

func walOff() {
	fmt.Println("== fsync off")
	e, _ := Restart(newDisk())
	e.fsync = false
	e.Update(7, 2)
	answer := e.Commit()
	cached := e.osCache[0]
	onDisk := e.disk.wal[len(e.disk.wal)-1]
	fmt.Printf("COMMIT: %q at once; record %d is in the OS cache, the WAL on disk ends at record %d\n", answer, cached.lsn, onDisk.lsn)

	d := e.PowerCut()
	fmt.Printf("power cut: the OS cache is wiped; WAL ends at record %d\n", d.wal[len(d.wal)-1].lsn)

	e, n := Restart(d)
	fmt.Printf("restart: replayed %d, wallet 7 = %d, the client was told %q\n", n, e.pages[7], answer)

	// Fix one: fsync on. The commit waits for one small sequential flush.
	e, _ = Restart(newDisk())
	e.Update(7, 2)
	e.Commit()
	e, _ = Restart(e.PowerCut())
	fmt.Printf("fix, fsync on: after the power cut wallet 7 = %d\n", e.pages[7])

	// Fix two: synchronous_commit = off. The answer comes before the flush.
	e, _ = Restart(newDisk())
	e.syncCommit = false
	e.Update(7, 2)
	e.Commit()
	e.Flush() // the WAL writer, a moment later
	e, _ = Restart(e.PowerCut())
	fmt.Printf("synchronous_commit = off, power cut after the WAL writer: wallet 7 = %d\n", e.pages[7])

	e.syncCommit = false // still off after the restart
	e.Update(7, 5)
	e.Commit()
	e, n = Restart(e.PowerCut()) // power cut before the WAL writer ran
	fmt.Printf("power cut before the WAL writer: replayed %d, wallet 7 = %d: the last commit is lost, the log stays whole\n", n, e.pages[7])
}

func main() {
	walOn()
	fmt.Println()
	walOff()
}
