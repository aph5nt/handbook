// Stored procedures: round trips, lock time and the permission boundary.
//
// A tiny model database runs the five statements of one transfer, either
// one round trip each from the app or inside one CALL. The clock is
// simulated: a round trip costs rttMicros, a statement stmtMicros.
// Part 2 shows the cost: one line in the trace, versions that drift, and
// the fix, a small procedure with the rules back in the app.
// Run: go run .
package main

import (
	"errors"
	"fmt"
)

const rttMicros = 2000 // one network round trip, a model value
const stmtMicros = 50  // one statement inside the database, a model value

var errNotEnough = errors.New("refused: balance below amount")

type DB struct {
	clock    int // simulated microseconds
	trips    int
	stmts    int
	lockedAt int
	held     int
	accounts map[string]int
	ledger   []string
	grants   map[string]map[string]bool // role -> privileges
	trace    []string
}

// Stmt is one statement and the privilege it needs.
type Stmt struct {
	SQL   string
	Needs string
	Run   func(d *DB) error
}

func newDB() *DB {
	return &DB{
		accounts: map[string]int{"A": 1000, "B": 1000},
		grants: map[string]map[string]bool{
			"owner":  {"UPDATE accounts": true, "SELECT accounts": true, "INSERT ledger": true},
			"app_rw": {"UPDATE accounts": true, "SELECT accounts": true, "INSERT ledger": true},
			"app":    {"EXECUTE transfer_funds": true},
		},
	}
}

func transferStmts(from, to string, amount int) []Stmt {
	return []Stmt{
		{"SELECT * FROM accounts WHERE id IN (A, B) FOR UPDATE", "UPDATE accounts",
			func(d *DB) error { d.lockedAt = d.clock; return nil }},
		{"SELECT balance FROM accounts WHERE id = A", "SELECT accounts",
			func(d *DB) error {
				if d.accounts[from] < amount {
					return errNotEnough
				}
				return nil
			}},
		{"UPDATE accounts SET balance = balance - amount WHERE id = A", "UPDATE accounts",
			func(d *DB) error { d.accounts[from] -= amount; return nil }},
		{"UPDATE accounts SET balance = balance + amount WHERE id = B", "UPDATE accounts",
			func(d *DB) error { d.accounts[to] += amount; return nil }},
		{"INSERT INTO ledger VALUES (A, B, amount)", "INSERT ledger",
			func(d *DB) error { d.ledger = append(d.ledger, fmt.Sprintf("%s->%s %d", from, to, amount)); return nil }},
	}
}

// roundTrip is one message from the app to the database and back.
func (d *DB) roundTrip(what string) {
	d.trips++
	d.clock += rttMicros
	d.trace = append(d.trace, "app -> db: "+what)
}

// run checks the role's privilege, then runs the statement next to the data.
func (d *DB) run(role string, s Stmt) error {
	if !d.grants[role][s.Needs] {
		return fmt.Errorf("permission denied: %s has no %s", role, s.Needs)
	}
	d.clock += stmtMicros
	d.stmts++
	return s.Run(d)
}

// commit releases the row locks and records how long they were held.
func (d *DB) commit() { d.held = d.clock - d.lockedAt }

// AppTransfer sends each statement on its own round trip.
func AppTransfer(d *DB, role, from, to string, amount int) error {
	for _, s := range transferStmts(from, to, amount) {
		d.roundTrip(s.SQL)
		if err := d.run(role, s); err != nil {
			return err
		}
	}
	d.commit()
	return nil
}

// CallTransfer is CALL transfer_funds: one round trip, and the statements
// run with the owner's rights (SECURITY DEFINER), so the caller needs
// only EXECUTE.
func CallTransfer(d *DB, role, from, to string, amount int) error {
	d.roundTrip("CALL transfer_funds")
	if !d.grants[role]["EXECUTE transfer_funds"] {
		return fmt.Errorf("permission denied: %s has no EXECUTE transfer_funds", role)
	}
	for _, s := range transferStmts(from, to, amount) {
		if err := d.run("owner", s); err != nil {
			return err
		}
	}
	d.commit()
	return nil
}

// ruleStmts are the business rules that drifted into the procedure.
func ruleStmts() []Stmt {
	return []Stmt{
		{"SELECT fee FROM fee_table", "SELECT accounts", func(d *DB) error { return nil }},
		{"SELECT SUM(amount) FROM ledger WHERE day = today", "SELECT accounts", func(d *DB) error { return nil }},
		{"SELECT 1 FROM fraud_list WHERE id = B", "SELECT accounts", func(d *DB) error { return nil }},
	}
}

// payout is what B receives under each deployed version of the procedure.
var payout = map[int]func(amount int) int{
	38: func(amount int) int { return amount - 1 },     // fee 1
	41: func(amount int) int { return amount - 2 - 2 }, // fee raised to 2, charged twice by mistake
}

var fraudList = map[string]bool{"M": true}

// appRules is the fix: limits and fraud checks in app code, where tests,
// logs and traces reach them.
func appRules(to string, amount int) error {
	if amount > 10000 {
		return errors.New("refused: over the daily limit")
	}
	if fraudList[to] {
		return errors.New("refused: receiver on the fraud list")
	}
	return nil
}

func pros() {
	d := newDB()
	if err := AppTransfer(d, "app_rw", "A", "B", 100); err != nil {
		fmt.Println("app transfer:", err)
	}
	fmt.Printf("logic in the app: %d round trips, locks held %d us\n", d.trips, d.held)

	d = newDB()
	if err := CallTransfer(d, "app", "A", "B", 100); err != nil {
		fmt.Println("call:", err)
	}
	fmt.Printf("stored procedure: %d round trip, locks held %d us, A=%d B=%d ledger=%d\n",
		d.trips, d.held, d.accounts["A"], d.accounts["B"], len(d.ledger))

	debit := transferStmts("A", "B", 100)[2]
	fmt.Println("app sends UPDATE accounts directly:", d.run("app", debit))
}

func cons() {
	d := newDB()
	_ = CallTransfer(d, "app", "A", "B", 100)
	fmt.Printf("app trace: %d line: %q, %d us\n", len(d.trace), d.trace[0], d.clock)

	lean := len(transferStmts("A", "B", 100))
	fat := lean + len(ruleStmts())
	fmt.Printf("statements inside the database per transfer: %d, with the rules %d\n", lean, fat)

	deployed := map[string]int{"staging": 41, "production": 38}
	for _, env := range []string{"production", "staging"} {
		v := deployed[env]
		fmt.Printf("%s runs v%d: B receives %d of 100\n", env, v, payout[v](100))
	}

	d = newDB()
	err := appRules("B", 100)
	if err == nil {
		err = CallTransfer(d, "app", "A", "B", 100)
	}
	ok := err == nil && d.accounts["A"] == 900 && d.accounts["B"] == 1100 && len(d.ledger) == 1
	fmt.Printf("fix: %d statements inside; check against the database: A=%d B=%d ledger=%d ok=%v\n",
		d.stmts, d.accounts["A"], d.accounts["B"], len(d.ledger), ok)
	fmt.Println("fix: app rule for receiver M:", appRules("M", 100))
}

func main() {
	fmt.Println("== one call, not five")
	pros()
	fmt.Println("== the cost, years later")
	cons()
}
