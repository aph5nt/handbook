// ETL versus ELT: the same euro bug, found a month later. ETL threw the raw
// orders away, so it cannot rebuild. ELT kept them, so a fixed model reruns.
package main

import "fmt"

type Order struct {
	ID       int
	Day      int // day of the month, 1 to 30
	Amount   float64
	Currency string
	IsTest   bool
	Email    string
}

const (
	eurRateWrong = 1.00 // the bug: euros counted as dollars
	eurRateRight = 1.10
)

// extract reads the month's orders from Postgres: 10,000 raw rows.
func extract() []Order {
	rows := make([]Order, 0, 10000)
	for i := 0; i < 10000; i++ {
		cur := "USD"
		if i%4 == 0 {
			cur = "EUR"
		}
		rows = append(rows, Order{ID: i, Day: i%30 + 1, Amount: 20, Currency: cur, IsTest: i%50 == 0, Email: fmt.Sprintf("c%d@shop.example", i)})
	}
	return rows
}

// etlNight transforms outside the warehouse and loads only the result.
// The raw rows are dropped when the function returns.
func etlNight(rows []Order, eurRate float64) map[int]float64 {
	revenue := map[int]float64{}
	for _, r := range rows {
		if r.IsTest {
			continue // drop test orders
		}
		usd := r.Amount
		if r.Currency == "EUR" {
			usd = r.Amount * eurRate // convert currency
		}
		revenue[r.Day] += usd // only this sum is loaded
	}
	return revenue // raw rows are discarded here
}

// The ELT model is SQL, kept in git and run inside the warehouse.
const eltSQL = `-- load: copy the raw rows in, unchanged
INSERT INTO raw_orders SELECT * FROM postgres_orders_month;

-- model revenue_by_day, v2, kept in git and run in the warehouse
INSERT OVERWRITE revenue_by_day
SELECT day,
       sum(CASE currency WHEN 'EUR' THEN amount * 1.10   -- v1 had 1.00: the bug
                         ELSE amount END) AS revenue
FROM   raw_orders
WHERE  NOT is_test
GROUP  BY day;

-- raw tables hold personal data: only the pipeline may read them
REVOKE SELECT ON raw_orders FROM ROLE analyst;
GRANT  SELECT ON raw_orders TO ROLE pipeline;`

// runModel is what the SQL above computes, over the kept raw table.
func runModel(raw []Order, eurRate float64) map[int]float64 {
	return etlNight(raw, eurRate) // same rule, but the input is the raw table
}

func total(m map[int]float64) float64 {
	t := 0.0
	for _, v := range m {
		t += v
	}
	return t
}

func main() {
	rows := extract()
	fmt.Printf("extract: %d orders\n", len(rows))

	// ETL: transform outside, load the result, drop the raw rows.
	etl := etlNight(rows, eurRateWrong)
	fmt.Printf("ETL loaded revenue: %.2f USD (euro bug inside)\n", total(etl))
	rows = nil // the night job ends, the raw input is gone
	fmt.Printf("ETL rebuild a month later: raw rows kept = %d, cannot rebuild\n", len(rows))

	// ELT: load raw first, transform inside with SQL.
	raw := extract() // loaded unchanged into raw_orders
	fmt.Printf("ELT raw_orders: %d rows, unchanged\n", len(raw))
	v1 := runModel(raw, eurRateWrong)
	fmt.Printf("ELT model v1: %.2f USD (same bug)\n", total(v1))
	v2 := runModel(raw, eurRateRight)
	fmt.Printf("ELT model v2 rerun over raw rows: %.2f USD (fixed)\n", total(v2))
	fmt.Printf("ELT trap: raw_orders still holds %d emails, grant it to the pipeline only\n", len(raw))
	_ = eltSQL
}
