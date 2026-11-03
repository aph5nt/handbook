// Star, snowflake and wide table: the same report, volume per exchange,
// over the same swaps. Counts the joins and shows what a rename does.
package main

import (
	"fmt"
	"sort"
)

// The SQL each shape runs. The Go below does the same work in memory.
const starSQL = `-- star: the pool dimension is flat
SELECT p.exchange_name,
       sum(f.amount) AS volume
FROM   swap_fact f
JOIN   pool_dim p ON p.pool_id = f.pool_id      -- join 1
WHERE  f.day >= current_date - INTERVAL 30 DAY
GROUP  BY p.exchange_name;

-- a rename touches every pool row of that exchange
UPDATE pool_dim SET exchange_name = 'Aero'
WHERE  exchange_name = 'Velodrome';             -- 10 rows`

const snowflakeSQL = `-- snowflake: exchange moved to its own table
SELECT e.name,
       sum(f.amount) AS volume
FROM   swap_fact f
JOIN   pool_dim p     ON p.pool_id = f.pool_id          -- join 1
JOIN   exchange_dim e ON e.exchange_id = p.exchange_id  -- join 2
WHERE  f.day >= current_date - INTERVAL 30 DAY
GROUP  BY e.name;

-- a rename is one row
UPDATE exchange_dim SET name = 'Aero' WHERE exchange_id = 7;

-- fix: flatten back into a star for reads
CREATE TABLE pool_dim_flat AS
SELECT p.pool_id, e.name AS exchange_name, p.fee
FROM   pool_dim p JOIN exchange_dim e ON e.exchange_id = p.exchange_id;`

const wideSQL = `-- wide table: exchange name copied into every swap row at write time
INSERT INTO swap_wide
SELECT f.pool_id, f.amount, f.day, p.exchange_name, p.fee
FROM   swap_fact f JOIN pool_dim p ON p.pool_id = f.pool_id;

-- the report: zero joins
SELECT exchange_name, sum(amount) FROM swap_wide GROUP BY exchange_name;

-- after a rename, old rows keep 'Velodrome': two lines for one exchange
-- fix: rebuild the derived table from the star
INSERT OVERWRITE swap_wide
SELECT f.pool_id, f.amount, f.day, p.exchange_name, p.fee
FROM   swap_fact f JOIN pool_dim p ON p.pool_id = f.pool_id;`

type Swap struct {
	PoolID int
	Amount float64
}

type FlatPool struct{ Exchange string } // star: name in the pool row
type SnowPool struct{ ExchangeID int }  // snowflake: only an id
type WideSwap struct {                  // wide: name copied into the fact
	Amount   float64
	Exchange string
}

func report(m map[string]float64, joins int, label string) {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	fmt.Printf("%-10s joins %d:", label, joins)
	for _, n := range names {
		fmt.Printf("  %s %.0f", n, m[n])
	}
	fmt.Println()
}

func main() {
	// pools 1..10 belong to Velodrome (exchange 7), pool 11 to Uniswap (exchange 8)
	flat := map[int]FlatPool{}
	snow := map[int]SnowPool{}
	exchange := map[int]string{7: "Velodrome", 8: "Uniswap"}
	for id := 1; id <= 10; id++ {
		flat[id] = FlatPool{"Velodrome"}
		snow[id] = SnowPool{7}
	}
	flat[11] = FlatPool{"Uniswap"}
	snow[11] = SnowPool{8}
	swaps := []Swap{{1, 100}, {4, 50}, {11, 30}}

	// star: one join, fact to pool
	star := map[string]float64{}
	for _, s := range swaps {
		star[flat[s.PoolID].Exchange] += s.Amount // join 1
	}
	report(star, 1, "star")

	// snowflake: two joins, fact to pool, pool to exchange
	sf := map[string]float64{}
	for _, s := range swaps {
		p := snow[s.PoolID]                    // join 1
		sf[exchange[p.ExchangeID]] += s.Amount // join 2
	}
	report(sf, 2, "snowflake")

	// a rename: how many rows change?
	rowsStar := 0
	for id, p := range flat {
		if p.Exchange == "Velodrome" {
			flat[id] = FlatPool{"Aero"}
			rowsStar++
		}
	}
	exchange[7] = "Aero" // snowflake: one row
	fmt.Printf("rename Velodrome to Aero: star changes %d pool rows, snowflake changes 1 exchange row\n", rowsStar)

	// wide table: names were copied in at write time, before the rename
	wide := []WideSwap{{100, "Velodrome"}, {50, "Velodrome"}, {30, "Uniswap"}}
	wide = append(wide, WideSwap{70, flat[2].Exchange}) // a new swap after the rename
	w := map[string]float64{}
	for _, s := range wide {
		w[s.Exchange] += s.Amount
	}
	report(w, 0, "wide")
	fmt.Println("wide table shows one exchange as two lines: Aero and Velodrome")

	// fix: rebuild the wide table from the star
	poolOf := []int{1, 4, 11, 2}
	w = map[string]float64{}
	for k, s := range wide {
		w[flat[poolOf[k]].Exchange] += s.Amount
	}
	report(w, 0, "rebuilt")
	_ = starSQL + snowflakeSQL + wideSQL
}
