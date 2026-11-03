// Golden rows and a sealed fixture: replay recorded node responses through
// the decoder, compare with frozen golden rows, refuse a changed bundle.
// The second half shows why a mock of the warehouse proves nothing.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
)

// Log is one recorded swap log, exactly as the node returned it.
type Log struct {
	Block    int    `json:"block"`
	Index    int    `json:"log"`
	Token    string `json:"token"`
	RawValue string `json:"raw"` // integer amount, before decimals
}

// Fixture is the sealed bundle: recorded logs plus decimals() replies.
type Fixture struct {
	From, To int
	Logs     []Log
	Decimals map[string]int // token -> its decimals() reply, recorded
}

type Row struct {
	Block, Index int
	Amount       string
	Symbol       string
}

var symbols = map[string]string{"0xusdc": "USDC", "0xusdce": "USDC", "0xweth": "WETH"}

func seal(f Fixture) string {
	b, _ := json.Marshal(f)
	return fmt.Sprintf("%x", sha256.Sum256(b))[:4]
}

// Load refuses a bundle whose hash does not match its seal.
func Load(f Fixture, want string) (Fixture, error) {
	if got := seal(f); got != want {
		return Fixture{}, fmt.Errorf("fixture hash %s, sealed %s: refuse", got, want)
	}
	return f, nil
}

// A decimals source: the old code, the fix, a mock, or the real engine.
type DecimalsFn func(token string) (int, bool)

func Decode(f Fixture, decimals DecimalsFn) []Row {
	var rows []Row
	for _, l := range f.Logs {
		d, ok := decimals(l.Token)
		if !ok {
			d = 5 // the bug: an unknown token falls back to a guess
		}
		raw, _ := new(big.Rat).SetString(l.RawValue)
		amt := raw.Quo(raw, new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(d)), nil)))
		rows = append(rows, Row{l.Block, l.Index, amt.FloatString(1), symbols[l.Token]})
	}
	return rows
}

// Compare is the check: every row equal to its golden row, or fail.
func Compare(got, golden []Row) error {
	for i := range golden {
		if got[i] != golden[i] {
			return fmt.Errorf("not equal at %d log %d: got %s %s, golden %s %s",
				golden[i].Block, golden[i].Index, got[i].Amount, got[i].Symbol, golden[i].Amount, golden[i].Symbol)
		}
	}
	return nil
}

func main() {
	fx := Fixture{From: 9500, To: 9600,
		Logs: []Log{
			{9512, 3, "0xusdc", "40000000"},
			{9547, 1, "0xusdce", "12500000"}, // bridged USDC, 6 decimals too
			{9583, 0, "0xweth", "3200000000000000000"},
			{9599, 2, "0xusdc", "7000000"},
		},
		Decimals: map[string]int{"0xusdc": 6, "0xusdce": 6, "0xweth": 18},
	}
	sealed := seal(fx)
	fmt.Println("recorded blocks", fx.From, "to", fx.To, "sealed with hash", sealed)

	golden := []Row{{9512, 3, "40.0", "USDC"}, {9547, 1, "12.5", "USDC"}, {9583, 0, "3.2", "WETH"}, {9599, 2, "7.0", "USDC"}}

	f, _ := Load(fx, sealed)
	// the old decoder reads decimals from the warehouse tokens table, by address;
	// the throwaway StarRocks holds the same rows as the lane: none for 0xusdce
	tokensTable := func(t string) (int, bool) { d, ok := map[string]int{"0xusdc": 6, "0xweth": 18}[t]; return d, ok }
	fmt.Println("old decoder:", Compare(Decode(f, tokensTable), golden))

	recorded := func(t string) (int, bool) { d, ok := f.Decimals[t]; return d, ok } // the fix
	fmt.Println("fixed decoder:", Compare(Decode(f, recorded), golden), "-> equal: merge")

	tampered := fx
	tampered.Logs = append([]Log(nil), fx.Logs...)
	tampered.Logs[1].RawValue = "1250000"
	_, err := Load(tampered, sealed)
	fmt.Println("one byte changed:", err)

	// the trap: a mock warehouse answers by symbol, the author's guess
	mock := func(t string) (int, bool) { return map[string]int{"USDC": 6, "WETH": 18}[symbols[t]], true }
	fmt.Println("old decoder on the mock:", Compare(Decode(f, mock), golden), "-> the test passes")
	// the real engine has no tokens row for the bridged USDC: no row, no value
	fmt.Println("old decoder on the real engine:", Compare(Decode(f, tokensTable), golden))
}
