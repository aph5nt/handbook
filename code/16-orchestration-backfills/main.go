// Orchestration and backfills: a nightly DAG run for a logical date, a
// retry, and a backfill of 1 to 9 September. Idempotent tasks replace their
// own day. Appending tasks double rows and a clock-reading task fetches the
// wrong day.
package main

import (
	"errors"
	"fmt"
)

const rowsPerDay = 24 // one volume row per hour

// Warehouse holds daily_volume rows per day, tagged with the source day.
type Warehouse struct{ days map[int][]int }

// buildReplace is INSERT OVERWRITE ... PARTITION (day = D): it replaces the day.
func (w *Warehouse) buildReplace(day int, src int) {
	rows := make([]int, rowsPerDay)
	for i := range rows {
		rows[i] = src
	}
	w.days[day] = rows // overwrite, never append
}

// buildAppend is a plain INSERT: it adds rows to whatever the day holds.
func (w *Warehouse) buildAppend(day int, src int) {
	for i := 0; i < rowsPerDay; i++ {
		w.days[day] = append(w.days[day], src)
	}
}

// runTask retries a task up to 3 times, as the orchestrator does.
func runTask(name string, try func(attempt int) error) {
	for attempt := 1; attempt <= 3; attempt++ {
		if err := try(attempt); err == nil {
			return
		}
		fmt.Printf("  %s failed, retry %d of 3\n", name, attempt)
	}
}

// nightly is the DAG: extract -> load -> (build, quality) -> refresh.
// Every task gets the logical date D, never the clock.
func nightly(w *Warehouse, d int) {
	src := d // extract(D)
	runTask("build", func(attempt int) error {
		w.buildReplace(d, src)
		return errOnce() // the first try times out after writing
	})
	// quality(D) and refresh(D) read the day; they write nothing here
}

// nightlyAppend is the trap DAG: build appends and extract may read the clock.
func nightlyAppend(w *Warehouse, d int, readsClock bool) {
	today := 10
	src := d // extract(D)
	if readsClock {
		src = today // the bug: extract asks the clock
	}
	runTask("build", func(attempt int) error {
		w.buildAppend(d, src) // wrote, then failed: the retry writes again
		return errOnce()
	})
}

// failNext makes the next build attempt fail after it wrote its rows.
var failNext bool

func errOnce() error {
	if failNext {
		failNext = false
		return errors.New("warehouse timed out after writing")
	}
	return nil
}

func report(name string, w *Warehouse) {
	fmt.Printf("%s:", name)
	for d := 1; d <= 10; d++ {
		rows := w.days[d]
		tag := ""
		for _, src := range rows {
			if src != d {
				tag = fmt.Sprintf("(holds %d Sep)", src)
			}
		}
		fmt.Printf(" %d:%d%s", d, len(rows), tag)
	}
	fmt.Println()
}

func main() {
	good := &Warehouse{days: map[int][]int{}}
	for d := 1; d <= 9; d++ {
		good.buildReplace(d, d)
	}
	fmt.Println("idempotent DAG, run for 10 Sep with one Build failure:")
	failNext = true
	nightly(good, 10)
	report("rows per day", good)
	for d := 1; d <= 9; d++ { // backfill: the same DAG, older logical dates
		nightly(good, d)
	}
	report("after backfill 1 to 9 Sep", good)

	bad := &Warehouse{days: map[int][]int{}}
	for d := 1; d <= 9; d++ {
		bad.buildAppend(d, d)
	}
	fmt.Println("appending DAG, run for 10 Sep with one Build failure:")
	failNext = true
	nightlyAppend(bad, 10, false)
	report("rows per day", bad)
	for d := 1; d <= 9; d++ {
		nightlyAppend(bad, d, d == 3) // the 3 Sep run reads the clock
	}
	report("after backfill 1 to 9 Sep", bad)
}
