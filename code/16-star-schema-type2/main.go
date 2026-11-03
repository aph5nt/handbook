// Facts, dimensions and a type 2 label dimension. The SQL that StarRocks
// runs is kept as constants; the Go below does the same joins in memory so
// the answers of the lesson can be printed and checked.
package main

import "fmt"

// The SQL of the lesson, as run in the warehouse.
const (
	type1Change = `UPDATE dim_address_label SET label = 'exchange'
WHERE  address = '0x9f..';                          -- old value is gone`
	type2Change = `UPDATE dim_address_label
SET    valid_to = '2026-10-02', is_current = false
WHERE  address = '0x9f..' AND is_current;           -- close the old row
INSERT INTO dim_address_label VALUES
  ('0x9f..', 'exchange', '2026-10-03', '9999-12-31', true);`
	asOfJoin = `SELECT f.day, f.amount_in, d.label
FROM   fact_swaps f
JOIN   dim_address_label d
  ON   d.address = f.trader
 AND   f.day BETWEEN d.valid_from AND d.valid_to   -- the label of that day
WHERE  f.day BETWEEN '2026-08-01' AND '2026-08-31';`
	currentJoin = `SELECT f.day, f.amount_in, d.label
FROM   fact_swaps f
JOIN   dim_address_label d ON d.address = f.trader AND d.is_current;`
	copiedInsert = `INSERT INTO fact_swaps (day, trader, pool, amount_in, label)
SELECT ..., (SELECT label FROM dim_address_label
            WHERE address = trader AND is_current);`
	copiedReport = `SELECT day, amount_in, label FROM fact_swaps WHERE trader = '0x9f..';`
	dimFix       = `UPDATE dim_address_label SET label = 'market maker'
WHERE  address = '0x9f..' AND valid_from = '2026-10-03';`
	copiedFix = `UPDATE fact_swaps SET label = 'market maker'
WHERE  trader = '0x9f..';               -- no day filter: August changes too`
)

type Fact struct {
	Day    string // YYYY-MM-DD
	Trader string
	Amount int
	Label  string // only in the copied design
}

type Dim struct {
	Address, Label, From, To string
	Current                  bool
}

var facts = []Fact{{"2026-08-03", "0x9f..", 120, ""}, {"2026-08-09", "0x9f..", 45, ""}, {"2026-10-12", "0x9f..", 300, ""}}

// labelAsOf joins on the day: the dimension row whose dates contain it.
func labelAsOf(dims []Dim, addr, day string) string {
	for _, d := range dims {
		if d.Address == addr && d.From <= day && day <= d.To {
			return d.Label
		}
	}
	return "?"
}

// labelNow joins on the current flag.
func labelNow(dims []Dim, addr string) string {
	for _, d := range dims {
		if d.Address == addr && d.Current {
			return d.Label
		}
	}
	return "?"
}

func augustLabels(dims []Dim) []string {
	var out []string
	for _, f := range facts {
		if f.Day < "2026-09-01" {
			out = append(out, labelAsOf(dims, f.Trader, f.Day))
		}
	}
	return out
}

func main() {
	dims := []Dim{{"0x9f..", "unknown", "2026-01-01", "9999-12-31", true}}
	fmt.Println("before 3 Oct, August report:", augustLabels(dims))

	// type 1: overwrite in place
	t1 := []Dim{{"0x9f..", "exchange", "2026-01-01", "9999-12-31", true}}
	fmt.Println("type 1 after 3 Oct, August report:", augustLabels(t1), "(history rewritten)")

	// type 2: close the old row, add a new one
	dims[0].To, dims[0].Current = "2026-10-02", false
	dims = append(dims, Dim{"0x9f..", "exchange", "2026-10-03", "9999-12-31", true})
	fmt.Println("type 2 after 3 Oct, August report:", augustLabels(dims))
	fmt.Println("type 2, label known today:", labelNow(dims, "0x9f.."))

	// copied into the facts: the label is frozen in each row
	copied := append([]Fact(nil), facts...)
	for i := range copied {
		copied[i].Label = "unknown"
		if copied[i].Day >= "2026-10-03" {
			copied[i].Label = "exchange"
		}
	}
	fmt.Println("copied labels:", copied[0].Label, copied[1].Label, copied[2].Label)
	changed := 0
	for i := range copied { // UPDATE ... WHERE trader = '0x9f..' with no day filter
		if copied[i].Trader == "0x9f.." {
			copied[i].Label = "market maker"
			changed++
		}
	}
	fmt.Printf("correction rewrote %d fact rows, August now: %s\n", changed, copied[0].Label)

	// the fix: correct one dimension row instead
	dims[1].Label = "market maker"
	fmt.Println("fix in the dimension, August report:", augustLabels(dims), "today:", labelNow(dims, "0x9f.."))
}
