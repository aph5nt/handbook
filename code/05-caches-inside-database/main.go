// Caches inside the database, as a small model: a materialized view with one
// row per pool per day, and a query result cache keyed by the exact SQL text.
package main

import "fmt"

type swap struct {
	pool, day string
	volume    int
}

type DB struct {
	swaps       []swap
	rowsRead    int
	view        map[[2]string]int // materialized view: (pool, day) -> volume
	refreshedTo string
	resultCache map[string]int // SQL text -> answer
}

// scan is the full aggregate: every swap of the table is read.
func (d *DB) scan(pool, day string) int {
	sum := 0
	for _, s := range d.swaps {
		d.rowsRead++
		if s.pool == pool && s.day == day {
			sum += s.volume
		}
	}
	return sum
}

// Refresh reads one sealed day once and stores its rows in the view.
func (d *DB) Refresh(day string) {
	for _, s := range d.swaps {
		if s.day == day {
			d.rowsRead++
			d.view[[2]string{s.pool, s.day}] += s.volume
		}
	}
	d.refreshedTo = day
}

// ReadView is a point read of the rows on screen.
func (d *DB) ReadView(pool, day string) (int, bool) {
	d.rowsRead++
	v, ok := d.view[[2]string{pool, day}]
	return v, ok
}

// Query uses the result cache: the key is the exact SQL text.
func (d *DB) Query(sql, pool, day string) (int, string) {
	if v, ok := d.resultCache[sql]; ok {
		return v, "cached"
	}
	v := d.scan(pool, day)
	d.resultCache[sql] = v
	return v, "ran"
}

// Write adds swaps. Changed data drops the cached answers.
func (d *DB) Write(s swap) {
	d.swaps = append(d.swaps, s)
	d.resultCache = map[string]int{}
}

func newDB() *DB {
	d := &DB{view: map[[2]string]int{}, resultCache: map[string]int{}}
	for i := 0; i < 1000; i++ {
		d.swaps = append(d.swaps, swap{"pool-a", "2026-10-06", 2})
	}
	for i := 0; i < 9000; i++ {
		d.swaps = append(d.swaps, swap{"pool-a", "2026-10-05", 1})
	}
	return d
}

func main() {
	d := newDB()
	fmt.Println("== materialized view ==")
	for i := 0; i < 10; i++ {
		d.scan("pool-a", "2026-10-06")
	}
	fmt.Printf("  10 dashboard loads, no view: %d rows read\n", d.rowsRead)
	d.rowsRead = 0
	d.Refresh("2026-10-06")
	fmt.Printf("  refresh one sealed day: %d rows read, once\n", d.rowsRead)
	d.rowsRead = 0
	for i := 0; i < 10; i++ {
		d.ReadView("pool-a", "2026-10-06")
	}
	fmt.Printf("  10 loads from the view: %d rows read\n", d.rowsRead)
	v, ok := d.ReadView("pool-a", "2026-10-07")
	fmt.Printf("  today, 2026-10-07, refreshed to %s: volume %d, row exists=%v, a reader thinks trading stopped\n", d.refreshedTo, v, ok)

	fmt.Println("== query result cache ==")
	d = newDB()
	const sql = "SELECT sum(volume) FROM swaps WHERE day = '2026-10-06'"
	_, how := d.Query(sql, "pool-a", "2026-10-06")
	r := d.rowsRead
	fmt.Printf("  first load: %s, %d rows read\n", how, r)
	_, how = d.Query(sql, "pool-a", "2026-10-06")
	fmt.Printf("  same text: %s, %d more rows read\n", how, d.rowsRead-r)
	d.Write(swap{"pool-a", "2026-10-06", 2})
	_, how = d.Query(sql, "pool-a", "2026-10-06")
	fmt.Printf("  after a late write: %s, the cached answer was dropped\n", how)

	d = newDB()
	hits := 0
	for sec := 0; sec < 10; sec++ { // now() makes a new text on every run
		text := fmt.Sprintf("SELECT ... WHERE ts >= '2026-10-06 14:00:%02d'", sec)
		if _, h := d.Query(text, "pool-a", "2026-10-06"); h == "cached" {
			hits++
		}
	}
	fmt.Printf("  now() in the SQL: %d hits of 10 loads\n", hits)
	hits = 0
	for sec := 0; sec < 10; sec++ { // rounded to the hour: one text
		_, h := d.Query("SELECT ... WHERE ts >= '2026-10-06 14:00:00'", "pool-a", "2026-10-06")
		if h == "cached" {
			hits++
		}
	}
	fmt.Printf("  time rounded to the hour: %d hits of 10 loads\n", hits)
}
