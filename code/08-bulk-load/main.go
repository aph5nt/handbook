// An in-memory model of a warehouse load: labels, commits and rows. It opens no
// connection. db and client are in-memory fakes with the same methods as
// database/sql and net/http, so the caller code in the lesson panel runs as written.
package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Table struct {
	Rows     int
	Commits  int
	Segments int
	labels   map[string]bool
}

// Load commits a load. A label already seen is refused, and nothing is added.
func (t *Table) Load(label string, rows int) string {
	if label != "" && t.labels[label] {
		return "Label Already Exists"
	}
	if label != "" {
		t.labels[label] = true
	}
	t.Rows += rows
	t.Commits++
	t.Segments++
	return "Success"
}

func newTable() *Table { return &Table{labels: map[string]bool{}} }

type Row struct {
	Pool   int64
	Amount float64
	Tx     string
}

// fakeDB has the ExecContext method of *sql.DB. Each statement is one commit.
type fakeDB struct{ t *Table }

func (d fakeDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	d.t.Load("", len(args)/3)
	return nil, nil
}

// fakeLoad stands in for the StarRocks frontend. It commits the CSV body, then
// loses the first loseReplies replies, the way a network timeout does.
type fakeLoad struct {
	t           *Table
	loseReplies int
	verbose     bool
	attempt     int
}

func (f *fakeLoad) RoundTrip(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	status := f.t.Load(req.Header.Get("label"), strings.Count(string(body), "\n"))
	f.attempt++
	if f.verbose {
		fmt.Printf("  attempt %d: %s, reply lost = %v\n", f.attempt, status, f.loseReplies > 0)
	}
	if f.loseReplies > 0 {
		f.loseReplies--
		return nil, errors.New("i/o timeout: the reply was lost")
	}
	out, _ := json.Marshal(map[string]string{"Status": status})
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{},
		Body: io.NopCloser(bytes.NewReader(out)), Request: req}, nil
}

// never for a page of rows: one statement per row
func perRowInsert(ctx context.Context, db fakeDB, rows []Row) {
	for _, r := range rows {
		db.ExecContext(ctx, "INSERT INTO swaps VALUES (?, ?, ?)", r.Pool, r.Amount, r.Tx)
	}
}

// a trickle: one statement, many VALUES, under the text limit
func trickle(ctx context.Context, db fakeDB, args ...any) {
	db.ExecContext(ctx, "INSERT INTO swaps VALUES (?, ?, ?), (?, ?, ?)", args...)
}

// a page: Stream Load sends rows as data
func streamLoad(ctx context.Context, client *http.Client, csvBody io.Reader) (*http.Response, error) {
	url := "http://fe:8030/api/robinhood/swaps/_stream_load"
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, url, csvBody)
	req.Header.Set("format", "csv")
	resp, err := client.Do(req)
	return resp, err
}

// history: files, read in parallel by the warehouse
// LOAD LABEL robinhood.swaps_2026 (DATA INFILE("s3://bucket/swaps/*.parquet") ...)

const baseURL = "http://fe:8030/api/robinhood/"

var client = &http.Client{}

func loadPage(ctx context.Context, table string, page int, body []byte) error {
	label := fmt.Sprintf("%s-page-%d", table, page)
	for attempt := 0; attempt < 5; attempt++ {
		req, _ := http.NewRequestWithContext(ctx, http.MethodPut,
			baseURL+table+"/_stream_load", bytes.NewReader(body))
		req.Header.Set("label", label) // the same label on every attempt
		req.Header.Set("format", "csv")
		resp, err := client.Do(req)
		if err != nil {
			continue // a timeout means unknown: send it again
		}
		var r struct{ Status string }
		json.NewDecoder(resp.Body).Decode(&r)
		resp.Body.Close()
		switch r.Status {
		case "Success", "Label Already Exists":
			return nil // the page is in the table, once
		}
	}
	return fmt.Errorf("page %d: not loaded", page)
}

// loadPageNoLabel is the bug: the same retry loop with no label header.
func loadPageNoLabel(ctx context.Context, table string, body []byte) error {
	for attempt := 0; attempt < 5; attempt++ {
		req, _ := http.NewRequestWithContext(ctx, http.MethodPut,
			baseURL+table+"/_stream_load", bytes.NewReader(body))
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
		return nil
	}
	return errors.New("not loaded")
}

func main() {
	ctx := context.Background()
	const n = 10_000
	rows := make([]Row, n)
	var csv strings.Builder
	for i := range rows {
		rows[i] = Row{Pool: 431, Amount: 12.5, Tx: fmt.Sprintf("0x%04x", i)}
		fmt.Fprintf(&csv, "%d,%.1f,%s\n", rows[i].Pool, rows[i].Amount, rows[i].Tx)
	}

	perRow := newTable()
	perRowInsert(ctx, fakeDB{perRow}, rows)
	fmt.Printf("per-row INSERT: %d commits, %d segments\n", perRow.Commits, perRow.Segments)

	few := newTable()
	trickle(ctx, fakeDB{few}, 431, 12.5, "0x0001", 431, 3.0, "0x0002")
	fmt.Printf("multi-row INSERT of 2 rows: %d commit, %d rows\n", few.Commits, few.Rows)

	bulk := newTable()
	resp, err := streamLoad(ctx, &http.Client{Transport: &fakeLoad{t: bulk}}, strings.NewReader(csv.String()))
	if err == nil {
		resp.Body.Close()
	}
	fmt.Printf("Stream Load:    %d commit, %d segment, %d rows\n", bulk.Commits, bulk.Segments, bulk.Rows)

	fmt.Println("no label, the first reply is lost:")
	t := newTable()
	client = &http.Client{Transport: &fakeLoad{t: t, loseReplies: 1, verbose: true}}
	loadPageNoLabel(ctx, "swaps", []byte(csv.String()))
	fmt.Printf("  table holds %d rows after %d commits\n", t.Rows, t.Commits)

	fmt.Println("label swaps-page-81200, the first reply is lost:")
	t = newTable()
	client = &http.Client{Transport: &fakeLoad{t: t, loseReplies: 1, verbose: true}}
	err = loadPage(ctx, "swaps", 81200, []byte(csv.String()))
	fmt.Printf("  loadPage error = %v, table holds %d rows after %d commit\n", err, t.Rows, t.Commits)
}
