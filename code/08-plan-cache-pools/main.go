// An in-memory model of a plan cache and of connection pools. It needs no
// database: the cache is a map keyed by statement text, the estimate comes
// from a histogram map, and the pool part multiplies the numbers a database
// front end really sees. The pool code at the end is the lesson panel's code:
// it runs on database/sql against a small in-memory driver registered as
// "mysql", which counts the connections it opens and the prepares it gets.
package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

const loopMaxRows = 10_000 // nested loop up to this many rows, else hash join

var rowsByPool = map[string]int{"tiny pool": 200, "router pool": 50_000_000}
var mostCommon = map[string]bool{"router pool": true} // what the histogram knows

func choose(est int) string {
	if est <= loopMaxRows {
		return "nested loop"
	}
	return "hash join"
}

type Cache map[string]string

// execute: the cache is keyed by text only, so the first value decides.
func (c Cache) execute(text, value string) {
	plan, ok := c[text]
	if !ok {
		plan = choose(rowsByPool[value])
		c[text] = plan
		fmt.Printf("  %-12s compile for %d rows: %s, stored\n", value, rowsByPool[value], plan)
	}
	fmt.Printf("  %-12s run %s over %d rows (right plan: %s)\n", value, plan, rowsByPool[value], choose(rowsByPool[value]))
}

// executePerClass keys the cache by text plus the class of the value.
func (c Cache) executePerClass(text, value string) {
	class := "small"
	if mostCommon[value] {
		class = "big"
	}
	key := text + "|" + class
	plan, ok := c[key]
	if !ok {
		plan = choose(rowsByPool[value])
		c[key] = plan
	}
	fmt.Printf("  %-12s class %-5s plan %s over %d rows\n", value, class, plan, rowsByPool[value])
}

// fakeDriver is an in-memory database/sql driver. It answers count(*) with 42.
type fakeDriver struct {
	mu              sync.Mutex
	conns, prepares int
}

func (d *fakeDriver) Open(name string) (driver.Conn, error) {
	d.mu.Lock()
	d.conns++
	d.mu.Unlock()
	return &fakeConn{d: d}, nil
}

type fakeConn struct{ d *fakeDriver }

func (c *fakeConn) Prepare(query string) (driver.Stmt, error) {
	c.d.mu.Lock()
	c.d.prepares++
	c.d.mu.Unlock()
	return fakeStmt{}, nil
}
func (c *fakeConn) Close() error              { return nil }
func (c *fakeConn) Begin() (driver.Tx, error) { return nil, errors.New("no transactions") }

type fakeStmt struct{}

func (fakeStmt) Close() error                               { return nil }
func (fakeStmt) NumInput() int                              { return 1 }
func (fakeStmt) Exec([]driver.Value) (driver.Result, error) { return nil, errors.New("read only") }
func (fakeStmt) Query([]driver.Value) (driver.Rows, error)  { return &fakeRows{}, nil }

type fakeRows struct{ done bool }

func (r *fakeRows) Columns() []string { return []string{"count(*)"} }
func (r *fakeRows) Close() error      { return nil }
func (r *fakeRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	time.Sleep(20 * time.Millisecond) // a query takes a while, so calls overlap
	r.done = true
	dest[0] = int64(42)
	return nil
}

func countSwaps(ctx context.Context, dsn string, pools []string) error {
	// StarRocks speaks the MySQL protocol.
	db, err := sql.Open("mysql", dsn) // dsn points at the pooler
	if err != nil {
		return err
	}
	// pods x MaxOpenConns is what the server sees: 50 x 5 = 250.
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	// One Stmt object. database/sql prepares it again on each
	// connection the first time it runs there.
	stmt, err := db.PrepareContext(ctx,
		"SELECT count(*) FROM swaps WHERE pool = ?")
	if err != nil {
		return err
	}
	defer stmt.Close() // frees the copy on every connection

	var wg sync.WaitGroup
	for _, pool := range pools {
		wg.Add(1)
		go func(pool string) {
			defer wg.Done()
			var n int64
			if err := stmt.QueryRowContext(ctx, pool).Scan(&n); err != nil {
				fmt.Println(err)
			}
		}(pool)
	}
	wg.Wait()
	return nil
}

func main() {
	const q = "SELECT ... FROM swaps WHERE pool = ?"
	fmt.Println("Parameter sniffing, one plan per text:")
	c := Cache{}
	c.execute(q, "tiny pool")
	c.execute(q, "router pool")
	fmt.Println("Fix, one plan per class of value:")
	pc := Cache{}
	pc.executePerClass(q, "tiny pool")
	pc.executePerClass(q, "router pool")

	const statements = 3 // prepared statements each connection holds
	fmt.Println("Pools:")
	for _, pods := range []int{1, 50} {
		conns := pods * 20
		fmt.Printf("  %2d pods x pool 20 = %4d connections, %4d prepared statement copies\n", pods, conns, conns*statements)
	}
	cores := 8
	fmt.Printf("  fix: 50 pods x pool 5 = %d client connections into the pooler\n", 50*5)
	fmt.Printf("  pooler to the database: cores %d x 2 = %d connections, %d statement copies\n", cores, cores*2, cores*2*statements)

	d := &fakeDriver{}
	sql.Register("mysql", d)
	pools := make([]string, 10)
	for i := range pools {
		pools[i] = fmt.Sprintf("pool-%d", i)
	}
	if err := countSwaps(context.Background(), "app@tcp(pooler:6432)/robinhood", pools); err != nil {
		fmt.Println(err)
	}
	fmt.Printf("  one pod, MaxOpenConns 5, 10 queries at once: %d connections opened, one Stmt prepared %d times\n", d.conns, d.prepares)
}
