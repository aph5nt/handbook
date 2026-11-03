// An in-memory model of a client and a database, with counters for round trips
// and statements. It needs no network and no database. db and client are
// in-memory fakes with the same method shapes as database/sql and net/http, so
// the caller code in the lesson panel runs as written, and each call counts.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// fakeDB counts one round trip per call. Each order has `lines` lines.
type fakeDB struct {
	Trips, Rows, Segments int
	lines                 int
}

type fakeRows struct {
	vals []int64
	i    int
}

func (r *fakeRows) Next() bool { r.i++; return r.i <= len(r.vals) }
func (r *fakeRows) Scan(dest ...any) error {
	*dest[len(dest)-1].(*int64) = r.vals[r.i-1]
	return nil
}

type fakeRow struct{}

func (fakeRow) Scan(dest ...any) error { return nil }

// QueryContext returns ids: the orders of a list page, or the product of each line.
func (d *fakeDB) QueryContext(ctx context.Context, query string, args ...any) (*fakeRows, error) {
	d.Trips++
	n := d.lines
	if strings.Contains(query, "LIMIT") {
		n = args[0].(int)
	}
	r := &fakeRows{vals: make([]int64, n)}
	for i := range r.vals {
		r.vals[i] = int64(i + 1)
	}
	d.Rows += n
	return r, nil
}

func (d *fakeDB) QueryRowContext(ctx context.Context, query string, args ...any) fakeRow {
	d.Trips++
	d.Rows++
	return fakeRow{}
}

func (d *fakeDB) ExecContext(ctx context.Context, query string, args ...any) (any, error) {
	d.Trips++
	d.Segments++
	return nil, nil
}

// each call looks like a field read. Each one is a round trip
func orderPage(ctx context.Context, db *fakeDB, id int64) error {
	// the order and its lines, in one query
	rows, err := db.QueryContext(ctx,
		"SELECT l.product_id FROM orders o JOIN order_lines l ON l.order_id = o.id WHERE o.id = $1", id)
	if err != nil {
		return err
	}
	for rows.Next() {
		var pid int64
		rows.Scan(&pid)
		// inside the loop: the bug
		var name string
		db.QueryRowContext(ctx,
			"SELECT name FROM products WHERE id = $1", pid).Scan(&name)
	}
	return nil
}

// a list page: one query for the orders, then the order page for each
func listPage(ctx context.Context, db *fakeDB, n int) error {
	rows, err := db.QueryContext(ctx, "SELECT id FROM orders ORDER BY id LIMIT $1", n)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		orderPage(ctx, db, id)
	}
	return nil
}

// upFront asks for the orders, then all their lines and products in one IN query.
func upFront(ctx context.Context, db *fakeDB, n int) {
	db.QueryContext(ctx, "SELECT id, buyer FROM orders ORDER BY id LIMIT $1", n)
	db.Trips++ // SELECT ... FROM order_lines JOIN products ... WHERE order_id = ANY($1)
	db.Rows += n * db.lines
}

type Row struct {
	Tx, To string
	Amount float64
}

type Page struct {
	ID   int
	Rows []Row
}

// the bug: one statement per row
func writePerRow(ctx context.Context, db *fakeDB, page Page) {
	for _, r := range page.Rows {
		db.ExecContext(ctx, "INSERT INTO transfers VALUES (?, ?, ?)", r.Tx, r.To, r.Amount)
	}
}

// fakeLoad stands in for the StarRocks frontend. A label it has seen is
// refused, and the table key (tx) replaces rows. It can lose a reply after the
// commit, the way a timeout does.
type fakeLoad struct {
	stored      map[string]bool
	labels      map[string]bool
	loads       int
	loseReplies int
}

func (f *fakeLoad) RoundTrip(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	status := "Label Already Exists"
	if l := req.Header.Get("label"); !f.labels[l] {
		f.labels[l] = true
		f.loads++
		status = "Success"
		for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
			f.stored[strings.Split(line, ",")[0]] = true
		}
	}
	if f.loseReplies > 0 {
		f.loseReplies--
		return nil, errors.New("i/o timeout: the reply was lost")
	}
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Status": {status}},
		Body: io.NopCloser(strings.NewReader(status)), Request: req}, nil
}

const loadURL = "http://fe:8030/api/robinhood/transfers/_stream_load"

var client = &http.Client{}

func encodeCSV(rows []Row) io.Reader {
	var b bytes.Buffer
	for _, r := range rows {
		fmt.Fprintf(&b, "%s,%s,%.1f\n", r.Tx, r.To, r.Amount)
	}
	return &b
}

func checkStatus(resp *http.Response) error {
	if s := resp.Header.Get("Status"); s != "Success" && s != "Label Already Exists" {
		return fmt.Errorf("load failed: %s", s)
	}
	return nil
}

// the fix: one load per table per page
func writePage(ctx context.Context, page Page) error {
	body := encodeCSV(page.Rows) // sized under the load limit
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, loadURL, body)
	req.Header.Set("label", fmt.Sprintf("transfers-page-%d", page.ID))
	resp, err := client.Do(req)
	if err != nil {
		return err // the caller sends the same page again
	}
	defer resp.Body.Close()
	return checkStatus(resp)
}

func main() {
	ctx := context.Background()
	for _, c := range []struct{ orders, lines int }{{1, 5}, {1, 20}, {50, 20}} {
		d := &fakeDB{lines: c.lines}
		if c.orders == 1 {
			orderPage(ctx, d, 1001)
		} else {
			listPage(ctx, d, c.orders)
		}
		j := &fakeDB{lines: c.lines}
		upFront(ctx, j, c.orders)
		fmt.Printf("%2d orders x %2d lines: lazy loop %4d round trips, up front %d\n", c.orders, c.lines, d.Trips, j.Trips)
	}
	one := &fakeDB{lines: 20}
	one.QueryContext(ctx, "SELECT o.id, l.qty, p.name FROM orders o JOIN order_lines l ... JOIN products p ... WHERE o.id = 1001")
	fmt.Printf("one order, one JOIN: %d round trip, %d rows\n", one.Trips, one.Rows)
	j := &fakeDB{lines: 20}
	upFront(ctx, j, 100)
	fmt.Printf("100 orders x 20 lines: up front %d round trips, %d rows\n", j.Trips, j.Rows)

	const rows = 10_000
	page := Page{ID: 9506, Rows: make([]Row, rows)}
	for i := range page.Rows {
		page.Rows[i] = Row{Tx: fmt.Sprintf("0x%05x", i), To: "0xa1", Amount: 12.5}
	}
	a := &fakeDB{}
	writePerRow(ctx, a, page)
	fmt.Printf("one INSERT per row: %d statements, %d segments\n", a.Trips, a.Segments)

	f := &fakeLoad{stored: map[string]bool{}, labels: map[string]bool{}, loseReplies: 1}
	client = &http.Client{Transport: f}
	err := writePage(ctx, page)
	fmt.Printf("one Stream Load:    %d statement, %d segment, first reply lost = %v\n", f.loads, f.loads, err != nil)
	err = writePage(ctx, page) // a retry after a timeout
	fmt.Printf("after a retry:      %d rows stored, not %d, retry error = %v\n", len(f.stored), 2*rows, err)
}
