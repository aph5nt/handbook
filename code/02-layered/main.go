// Replays the layered lesson: an admin tool for the price oracle.
// A request goes controller -> service -> repository -> database and back.
// Then the sinkhole count, the open layer, and one rule kept in two places.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Token is one row of the tokens table.
type Token struct {
	Symbol  string `json:"symbol"`
	Name    string `json:"name"`
	Sources int    `json:"sources"` // price sources behind its price
}

var trace []string // the layers one request crossed, in order

// Database is the bottom layer: one table in memory.
type Database struct{ rows []Token }

// Select is the SQL: SELECT * FROM tokens WHERE sources >= min.
func (db *Database) Select(min int) []Token {
	trace = append(trace, "database")
	var out []Token
	for _, t := range db.rows {
		if t.Sources >= min {
			out = append(out, t)
		}
	}
	return out
}

// Repository holds the SQL. It calls only the database.
type Repository struct {
	db        *Database
	exportMin int // rule copy 2: the WHERE filter of the export query
}

func (r *Repository) ListTokens() []Token {
	trace = append(trace, "repository")
	return r.db.Select(0)
}

func (r *Repository) ExportPrices() []Token {
	trace = append(trace, "repository")
	return r.db.Select(r.exportMin) // rule copy 2, in SQL
}

func (r *Repository) GetToken(sym string) Token {
	for _, t := range r.ListTokens() {
		if t.Symbol == sym {
			return t
		}
	}
	return Token{}
}

func (r *Repository) RenameToken(sym, name string) {
	trace = append(trace, "repository", "database")
	for i := range r.db.rows {
		if r.db.rows[i].Symbol == sym {
			r.db.rows[i].Name = name
		}
	}
}

func (r *Repository) DeleteToken(sym string) { trace = append(trace, "repository", "database") }

// Service holds business rules. It calls only the repository.
type Service struct {
	repo      *Repository
	forwarded map[string]bool // methods that only passed the call on
}

func (s *Service) pass(method string) {
	trace = append(trace, "service")
	s.forwarded[method] = true // a sinkhole: no rule, no change
}

func (s *Service) ListTokens() []Token       { s.pass("ListTokens"); return s.repo.ListTokens() }
func (s *Service) GetToken(sym string) Token { s.pass("GetToken"); return s.repo.GetToken(sym) }
func (s *Service) DeleteToken(sym string)    { s.pass("DeleteToken"); s.repo.DeleteToken(sym) }
func (s *Service) ExportPrices() []Token     { s.pass("ExportPrices"); return s.repo.ExportPrices() }

// RenameToken is the one method with a real rule.
func (s *Service) RenameToken(sym, name string) error {
	trace = append(trace, "service")
	if name == "" || len(name) > 32 {
		return errors.New("name must be 1 to 32 characters")
	}
	s.repo.RenameToken(sym, name)
	return nil
}

// Controller speaks HTTP. It calls only the service, unless the service is open.
type Controller struct {
	svc        *Service
	repo       *Repository
	open       bool // open layer: simple reads skip the service
	minSources int  // rule copy 1: a filter in the controller
}

// GetTokens answers GET /tokens with JSON.
func (c *Controller) GetTokens() (string, int) {
	trace = append(trace, "controller")
	var rows []Token
	if c.open {
		rows = c.repo.ListTokens() // skip the service on purpose
	} else {
		rows = c.svc.ListTokens() // closed: only the layer below
	}
	var shown []Token
	for _, t := range rows {
		if t.Sources >= c.minSources { // rule copy 1
			shown = append(shown, t)
		}
	}
	b, _ := json.Marshal(shown)
	return string(b), len(shown)
}

func symbols(ts []Token) []string {
	var out []string
	for _, t := range ts {
		out = append(out, t.Symbol)
	}
	return out
}

func main() {
	db := &Database{rows: []Token{
		{"WETH", "Wrapped Ether", 6}, {"USDC", "USD Coin", 5}, {"HOOD", "Robinhood", 4},
		{"CIA", "Chain Insights", 3}, {"PEPE", "Pepe", 2},
	}}
	repo := &Repository{db: db}
	svc := &Service{repo: repo, forwarded: map[string]bool{}}
	ctl := &Controller{svc: svc, repo: repo}

	fmt.Println("tab 1: GET /tokens through closed layers")
	trace = nil
	body, n := ctl.GetTokens()
	fmt.Printf("  layers crossed: %s (%d)\n", strings.Join(trace, " -> "), len(trace))
	fmt.Printf("  rows back up as JSON: %d tokens, %d bytes\n", n, len(body))

	fmt.Println("tab 2: the sinkhole count")
	svc.forwarded = map[string]bool{}
	svc.ListTokens()
	svc.GetToken("WETH")
	svc.DeleteToken("PEPE")
	svc.ExportPrices()
	err := svc.RenameToken("CIA", "")
	fmt.Printf("  service methods: 5, only pass the call on: %d\n", len(svc.forwarded))
	fmt.Printf("  RenameToken(\"\") refused by its rule: %v\n", err)

	ctl.open = true
	trace = nil
	ctl.GetTokens()
	fmt.Printf("  open service layer: %s (%d)\n", strings.Join(trace, " -> "), len(trace))

	ctl.minSources, repo.exportMin = 3, 3
	_, shown := ctl.GetTokens()
	fmt.Printf("  rule at 3 sources: screen shows %d, export has %d\n", shown, len(svc.ExportPrices()))

	repo.exportMin = 5 // the SQL copy is updated, the controller copy is missed
	_, shown = ctl.GetTokens()
	export := svc.ExportPrices()
	var leaked []string
	for _, t := range db.rows {
		if t.Sources >= ctl.minSources && t.Sources < repo.exportMin {
			leaked = append(leaked, fmt.Sprintf("%s %d sources", t.Symbol, t.Sources))
		}
	}
	fmt.Printf("  rule at 5 sources, SQL updated only: screen shows %d, export has %d %v\n", shown, len(export), symbols(export))
	fmt.Printf("  prices shown that should be hidden: %d %v\n", shown-len(export), leaked)
}
