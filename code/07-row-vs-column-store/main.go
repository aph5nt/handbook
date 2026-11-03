// The same table stored two ways. The program counts the places each
// layout reads and writes, and the segments a column store collects.
package main

import (
	"fmt"
	"strconv"
	"strings"
)

// RowStore keeps each row as one piece, like PostgreSQL.
type RowStore struct {
	rows   [][]string
	index  map[string]int // key -> row number
	places int            // pieces read or written
}

// ColumnStore keeps each column as its own file, like StarRocks.
type ColumnStore struct {
	files  [][]string // files[column][row]
	index  map[string]int
	places int
}

func NewRowStore(rows [][]string) *RowStore {
	s := &RowStore{index: map[string]int{}}
	for _, r := range rows {
		s.Insert(r)
	}
	s.places = 0
	return s
}

func NewColumnStore(columns int, rows [][]string) *ColumnStore {
	s := &ColumnStore{files: make([][]string, columns), index: map[string]int{}}
	for _, r := range rows {
		s.Insert(r)
	}
	s.places = 0
	return s
}

// Get reads one row: one piece holds every column.
func (s *RowStore) Get(key string) []string {
	s.places++
	return s.rows[s.index[key]]
}

// Get reads one row: one place per column file, then stitches them.
func (s *ColumnStore) Get(key string) []string {
	r, row := s.index[key], []string{}
	for c := range s.files {
		s.places++
		row = append(row, s.files[c][r])
	}
	return row
}

// Insert appends one piece.
func (s *RowStore) Insert(row []string) {
	s.index[row[0]] = len(s.rows)
	s.rows = append(s.rows, row)
	s.places++
}

// Insert appends to every column file.
func (s *ColumnStore) Insert(row []string) {
	s.index[row[0]] = len(s.files[0])
	for c, v := range row {
		s.files[c] = append(s.files[c], v)
		s.places++
	}
}

// SumByDay on a row store: every row is read whole.
func (s *RowStore) SumByDay(day, amount int) (map[string]int, int) {
	sums, read := map[string]int{}, 0
	for _, row := range s.rows {
		read += len(row) // the whole piece comes off disk
		n, _ := strconv.Atoi(row[amount])
		sums[row[day]] += n
	}
	return sums, read
}

// SumByDay on a column store: only the two named files are opened.
func (s *ColumnStore) SumByDay(day, amount int) (map[string]int, int) {
	sums, read := map[string]int{}, 0
	for r := range s.files[day] {
		read += 2 // one value from each file
		n, _ := strconv.Atoi(s.files[amount][r])
		sums[s.files[day][r]] += n
	}
	return sums, read
}

// runs compresses a column file: equal neighbours become one value and a count.
func runs(file []string) string {
	out := []string{}
	for i := 0; i < len(file); {
		j := i
		for j < len(file) && file[j] == file[i] {
			j++
		}
		out = append(out, fmt.Sprintf("%s x%d", file[i], j-i))
		i = j
	}
	return strings.Join(out, ", ")
}

func wallets() {
	rows := [][]string{{"0xa1", "12", "4", "1 Oct", "-"}, {"0xb2", "0", "9", "2 Oct", "-"},
		{"0xc3", "75", "2", "2 Oct", "dex"}, {"0xd4", "3", "1", "3 Oct", "-"}}
	rs, cs := NewRowStore(rows), NewColumnStore(5, rows)
	row := rs.Get("0xc3")
	fmt.Printf("row store: balance of 0xc3 = %s, places read %d\n", row[1], rs.places)
	row = cs.Get("0xc3")
	fmt.Printf("column store: balance of 0xc3 = %s, places read %d, then stitched\n", row[1], cs.places)
	rs.places, cs.places = 0, 0
	rs.Insert([]string{"0xe5", "40", "7", "3 Oct", "-"}) // a new wallet
	cs.Insert([]string{"0xe5", "40", "7", "3 Oct", "-"})
	fmt.Printf("new wallet: row store %d append, column store %d appends\n", rs.places, cs.places)
}

func transfers() {
	rows := [][]string{{"1", "1 Oct", "a", "b", "10"}, {"2", "1 Oct", "b", "c", "20"},
		{"3", "2 Oct", "a", "c", "30"}, {"4", "2 Oct", "c", "d", "15"}, {"5", "3 Oct", "d", "a", "25"}}
	rs, cs := NewRowStore(rows), NewColumnStore(5, rows)
	sums, read := rs.SumByDay(1, 4)
	fmt.Printf("row store: SUM(amount) by day %v, values read %d, %d of 5 columns\n", sums, read, read/len(rows))
	sums, read = cs.SumByDay(1, 4)
	fmt.Printf("column store: SUM(amount) by day %v, values read %d, %d of 5 columns\n", sums, read, read/len(rows))
	fmt.Printf("day file compressed: %s\n", runs(cs.files[1]))
}

// segments runs minutes of loads. Each load makes one segment; compaction
// merges at most mergePerMinute segments into one, once a minute.
func segments(minutes, loadsPerMinute int) int {
	const mergePerMinute = 10
	seg := 0
	for m := 1; m <= minutes; m++ {
		seg += loadsPerMinute
		if seg > 1 {
			k := min(seg, mergePerMinute)
			seg = seg - k + 1
		}
	}
	return seg
}

func loads() {
	const maxVersions = 1000 // StarRocks refuses new loads above this per tablet
	segs := [][]string{}
	for _, tx := range []string{"t1", "t2", "t3", "t4", "t5"} {
		segs = append(segs, []string{tx}) // each INSERT writes its own segment
	}
	fmt.Printf("one row per insert: %d inserts, %d segments\n", 5, len(segs))
	fmt.Printf("one row per insert: 44 rows a minute, minute 1: %d segments\n", segments(1, 44))
	fmt.Printf("one row per insert: minute 2: %d segments\n", segments(2, 44))
	for m := 1; ; m++ {
		if n := segments(m, 44); n > maxVersions {
			fmt.Printf("one row per insert: minute %d: %d segments, over %d: new loads refused\n", m, n, maxVersions)
			break
		}
	}
	fmt.Printf("one batch per page: 2 pages a minute, minute 20: %d segment for 40 pages\n", segments(20, 2))
}

func main() {
	wallets()
	transfers()
	loads()
}
