// Command linkedin replays the one-log lesson: nine feeds, a rename that a
// feed swallows, then one log with an offset per consumer, and a consumer that
// stops and catches up.
package main

import "fmt"

type Record map[string]string

func Feeds(producers, consumers int) int { return producers * consumers }
func Links(producers, consumers int) int { return producers + consumers }

// Feed reads one field by name. A missing field drops the record, silently.
func Feed(rows []Record, field string) (out []string) {
	for _, r := range rows {
		if v, ok := r[field]; ok {
			out = append(out, v)
		}
	}
	return out
}

type Log struct{ Rows []Record }

func (l *Log) Append(r Record) { l.Rows = append(l.Rows, r) }

type Consumer struct {
	Name   string
	Offset int
}

func (c *Consumer) Read(l *Log, batch int) int {
	end := min(c.Offset+batch, len(l.Rows))
	n := end - c.Offset
	c.Offset = end
	return n
}

// Produce never looks at who reads, so a stopped consumer blocks nothing.
func Produce(l *Log, n int) {
	for i := 0; i < n; i++ {
		l.Append(Record{"n": fmt.Sprint(len(l.Rows))})
	}
}

func CatchUp(c *Consumer, l *Log, batch int) (total int) {
	for {
		n := c.Read(l, batch)
		if n == 0 {
			return total
		}
		total += n
	}
}

func main() {
	fmt.Println("before step 3: feeds:", Feeds(3, 3), "; with 10 and 10:", Feeds(10, 10))
	rows := []Record{{"member": "ana"}, {"member": "ben"}}
	fmt.Println("before step 4: before the rename:", len(Feed(rows, "member")), "records")
	renamed := []Record{{"member_id": "ana"}, {"member_id": "ben"}}
	fmt.Println("before step 4: after the rename:", len(Feed(renamed, "member")), "records, no error")

	fmt.Println("after step 3: links:", Links(3, 3), "; a fourth consumer:", Links(3, 4))
	l := &Log{}
	search := &Consumer{Name: "search"}
	recs := &Consumer{Name: "recommendations"}
	hadoop := &Consumer{Name: "hadoop"}
	Produce(l, 1000)
	search.Read(l, 1000)
	recs.Read(l, 1000)
	hadoop.Read(l, 1000)
	fmt.Println("after step 3: every offset:", search.Offset, recs.Offset, hadoop.Offset)
	Produce(l, 6000) // an hour of records, a model figure; Hadoop is down
	search.Read(l, 6000)
	recs.Read(l, 6000)
	fmt.Printf("after step 5: log end %d, search %d, recommendations %d, hadoop %d\n", len(l.Rows), search.Offset, recs.Offset, hadoop.Offset)
	got := CatchUp(hadoop, l, 2000)
	fmt.Printf("after step 6: hadoop read %d from offset 1000, now at %d\n", got, hadoop.Offset)
}
