// Row store versus column store: count the cells each layout reads to
// answer SUM(amount) WHERE day = '24 Sep' over the lesson's 8 transfers.
package main

import "fmt"

type Transfer struct {
	Day, Block, Sender, Receiver string
	Amount                       float64
}

var table = []Transfer{
	{"23 Sep", "9401", "0x3e..", "0x7a..", 4.0}, {"23 Sep", "9455", "0x7a..", "0x11..", 80},
	{"24 Sep", "9506", "0xab..", "0xcd..", 12.5}, {"24 Sep", "9507", "0x11..", "0x22..", 3.0},
	{"24 Sep", "9533", "0xab..", "0x90..", 40}, {"24 Sep", "9561", "0x5c..", "0xab..", 7.25},
	{"25 Sep", "9610", "0x90..", "0x3e..", 1.5}, {"25 Sep", "9688", "0x11..", "0x5c..", 22},
}

const columns = 5

// RowStore keeps whole rows together. An index on day lists row numbers.
type RowStore struct {
	rows    []Transfer
	byDay   map[string][]int
	byBlock map[string]int
}

// SumByDay reads every matching row whole: all five columns come off disk.
func (s RowStore) SumByDay(day string) (sum float64, cellsRead int) {
	for _, r := range s.byDay[day] { // the index jumps to the matching rows
		row := s.rows[r] // the whole row is read, every column
		cellsRead += columns
		sum += row.Amount
	}
	return sum, cellsRead
}

// Block is one piece of a column with its min and max day.
type Block struct {
	MinDay, MaxDay string
	Days           []string
	Amounts        []float64
}

// ColumnStore keeps each column apart, cut into blocks with statistics.
type ColumnStore struct{ blocks []Block }

// SumByDay skips blocks whose min and max cannot match, then reads only
// the day column (to check the filter) and the amount column (to add).
func (s ColumnStore) SumByDay(day string) (sum float64, cellsRead int, skipped int) {
	for _, b := range s.blocks {
		if day < b.MinDay || day > b.MaxDay { // min and max cannot match
			skipped++
			continue // pruned: not one byte read
		}
		for k, d := range b.Days { // read the day column
			cellsRead++
			if d == day {
				cellsRead++ // read the amount column
				sum += b.Amounts[k]
			}
		}
	}
	return sum, cellsRead, skipped
}

func main() {
	rs := RowStore{rows: table, byDay: map[string][]int{}, byBlock: map[string]int{}}
	cs := ColumnStore{}
	for i, t := range table {
		rs.byDay[t.Day] = append(rs.byDay[t.Day], i)
		rs.byBlock[t.Block] = i
		n := len(cs.blocks)
		if n == 0 || cs.blocks[n-1].MinDay != t.Day {
			cs.blocks = append(cs.blocks, Block{MinDay: t.Day, MaxDay: t.Day})
			n++
		}
		cs.blocks[n-1].Days = append(cs.blocks[n-1].Days, t.Day)
		cs.blocks[n-1].Amounts = append(cs.blocks[n-1].Amounts, t.Amount)
	}
	total := len(table) * columns
	sum, cells := rs.SumByDay("24 Sep")
	fmt.Printf("row store:    sum %.2f USDC, cells read %d of %d\n", sum, cells, total)
	sum, cells, skipped := cs.SumByDay("24 Sep")
	fmt.Printf("column store: sum %.2f USDC, cells read %d of %d, blocks skipped %d of %d\n", sum, cells, total, skipped, len(cs.blocks))
	// the failure: no day filter, every block and every amount is read
	cells = 0
	for _, b := range cs.blocks {
		cells += 2 * len(b.Days)
	}
	fmt.Printf("column store, no day filter: cells read %d of %d, blocks skipped 0\n", cells, total)
	s := rs
	// one record by key: the whole row, every cell wanted
	row := s.rows[s.byBlock["9507"]]
	fmt.Printf("row store, one record by key: block %s from %s, %d of %d cells wanted\n", row.Block, row.Sender, columns, columns)
	// a report with no day filter reads every row of the live table
	sum = 0
	for _, row := range s.rows {
		sum += row.Amount // writes wait behind it
	}
	fmt.Printf("row store, report on the live database: sum %.2f, cells read %d of %d\n", sum, total, total)
}
