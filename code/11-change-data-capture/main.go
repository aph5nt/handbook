// Command cdc replays the change data capture lesson: a dual write that a
// crash cuts in half, then one write and a reader that follows the log, a
// reader that resumes, and a reader that fell behind the log.
package main

import "fmt"

type Entry struct {
	Pos           int
	Key, Old, New string
}

type DB struct {
	Rows map[string]string
	Log  []Entry
	Next int
}

func (d *DB) Commit(key, val string) {
	d.Next++
	d.Log = append(d.Log, Entry{d.Next, key, d.Rows[key], val})
	d.Rows[key] = val
}

func (d *DB) Purge(before int) {
	for len(d.Log) > 0 && d.Log[0].Pos < before {
		d.Log = d.Log[1:]
	}
}

// DualWrite saves then sends. A crash between them loses the event.
func DualWrite(d *DB, topic *[]Entry, key, val string, crash bool) {
	d.Rows[key] = val
	if crash {
		return
	}
	*topic = append(*topic, Entry{0, key, "", val})
}

type Reader struct{ Saved int }

func (r *Reader) Run(d *DB, topic *[]Entry) (string, int) {
	if len(d.Log) > 0 && r.Saved+1 < d.Log[0].Pos {
		for k, v := range d.Rows {
			*topic = append(*topic, Entry{0, k, "", v})
		}
		r.Saved = d.Next
		return "snapshot", len(d.Rows)
	}
	n := 0
	for _, e := range d.Log {
		if e.Pos > r.Saved {
			*topic = append(*topic, e)
			r.Saved = e.Pos
			n++
		}
	}
	return "followed", n
}

func Upsert(view map[string]string, topic []Entry) {
	for _, e := range topic {
		view[e.Key] = e.New
	}
}

func main() {
	d := &DB{Rows: map[string]string{"member 7": "Engineer"}}
	var topic []Entry
	DualWrite(d, &topic, "member 7", "Staff Engineer", true)
	search := map[string]string{"member 7": "Engineer"}
	Upsert(search, topic)
	fmt.Printf("dual step 3-4: database %q, topic has %d events, search %q\n", d.Rows["member 7"], len(topic), search["member 7"])

	d = &DB{Rows: map[string]string{"member 7": "Engineer"}}
	topic, r := nil, &Reader{}
	d.Commit("member 7", "Staff Engineer")
	how, n := r.Run(d, &topic)
	fmt.Printf("cdc step 1-4: %s %d event: %+v, saved position %d\n", how, n, topic[0], r.Saved)
	Upsert(search, topic)
	fmt.Printf("cdc step 5: search %q\n", search["member 7"])
	how, n = r.Run(d, &topic)
	fmt.Printf("cdc: a restart at position %d: %s %d new events\n", r.Saved, how, n)
	for i := 0; i < 5; i++ {
		d.Commit("member 8", fmt.Sprintf("title %d", i))
	}
	d.Purge(4)
	how, n = r.Run(d, &topic)
	fmt.Printf("cdc step 6: saved position 1, oldest log entry 4: %s of %d rows\n", how, n)
}
