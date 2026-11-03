// Anti-corruption layer: one adapter per vendor turns a foreign price row
// into our own DayPrice at the border. Inside, nobody knows px or close.
package main

import (
	"fmt"
	"time"
)

// DayPrice is our own model. The domain sees only this.
type DayPrice struct {
	Token string
	Day   string // YYYY-MM-DD, UTC
	USD   float64
}

// VendorARow is vendor A's shape, in vendor A's words.
type VendorARow struct {
	Px  float64
	Ccy string
	Ts  int64 // seconds; after the change, milliseconds
}

// VendorBRow is vendor B's shape.
type VendorBRow struct {
	Close float64
	Date  string
}

// AdapterA translates vendor A. tsInMs follows the vendor's format change.
func AdapterA(token string, r VendorARow, tsInMs bool) DayPrice {
	sec := r.Ts
	if tsInMs {
		sec = r.Ts / 1000 // the only line the format change touched
	}
	day := time.Unix(sec, 0).UTC().Format("2006-01-02")
	return DayPrice{Token: token, Day: day, USD: r.Px}
}

// AdapterB translates vendor B: different names, same DayPrice out.
func AdapterB(token string, r VendorBRow) DayPrice {
	return DayPrice{Token: token, Day: r.Date, USD: r.Close}
}

// NoLayer is the failure: pricing reads the vendor's ts directly, as seconds.
func NoLayer(r VendorARow) string {
	return fmt.Sprintf("year %d", time.Unix(r.Ts, 0).UTC().Year())
}

func main() {
	before := VendorARow{Px: 0.98, Ccy: "usd", Ts: 1790812800}
	fmt.Printf("adapter A: %+v\n", AdapterA("USDC", before, false))
	fmt.Printf("adapter B: %+v\n", AdapterB("USDC", VendorBRow{Close: 0.98, Date: "2026-10-01"}))
	after := VendorARow{Px: 0.98, Ccy: "usd", Ts: 1790812800000} // vendor A now sends ms
	fmt.Println("no layer reads the day as:", NoLayer(after))
	fmt.Printf("adapter A, fixed: %+v\n", AdapterA("USDC", after, true))
}
