// Data observability: four monitors on the swap facts table, plus a daily
// reconciliation. Each scenario replays one lesson step. The trap watches
// only the job's exit status.
package main

import (
	"fmt"
	"sort"
)

// normal rows per hour, in hundreds, hours 1..12
var normal = []int{9, 10, 9, 11, 10, 9, 10, 10, 9, 10, 11, 10}

const (
	freshnessLimitHours = 1   // newest row may trail the chain head by this
	volumeFloor         = 0.6 // an hour below 60 % of the usual hour is a cliff
	nullShareLimit      = 0.2 // more than 20 % empty amount_usd is wrong
)

func median(xs []int) float64 {
	s := append([]int(nil), xs...)
	sort.Ints(s)
	return float64(s[len(s)/2])
}

// Freshness: hours since the newest hour that holds rows.
func Freshness(rows []int) (hoursBehind int, alert bool) {
	for h := len(rows) - 1; h >= 0; h-- {
		if rows[h] > 0 {
			hoursBehind = len(rows) - 1 - h
			break
		}
	}
	return hoursBehind, hoursBehind > freshnessLimitHours
}

// Volume: each hour against the median of the usual day.
func Volume(rows []int) (hour int, ratio float64, alert bool) {
	m := median(normal)
	for h, r := range rows {
		if r > 0 && float64(r)/m < volumeFloor {
			return h + 1, float64(r) / m, true
		}
	}
	return 0, 1, false
}

// Schema: any column whose type differs from the last seen schema.
func Schema(last, now map[string]string) []string {
	var changed []string
	for col, t := range now {
		if last[col] != t {
			changed = append(changed, col+": "+last[col]+" -> "+t)
		}
	}
	return changed
}

// Distribution: share of rows in an hour with no USD value.
func Distribution(empty, total int) (share float64, alert bool) {
	share = float64(empty) / float64(total)
	return share, share > nullShareLimit
}

// Reconcile: rows per block range against the chain's own count.
func Reconcile(ours, chain int) bool { return ours == chain }

func main() {
	stall := append(append([]int(nil), normal[:10]...), 0, 0)
	h, a := Freshness(stall)
	fmt.Printf("freshness: newest rows %d hours behind, alert %v\n", h, a)

	cliff := append(append([]int(nil), normal[:10]...), 5, 10)
	hr, ratio, a2 := Volume(cliff)
	fmt.Printf("volume: hour %d at %.0f %% of usual, alert %v\n", hr, ratio*100, a2)

	fmt.Println("schema:", Schema(map[string]string{"amount_in": "decimal(38,9)"}, map[string]string{"amount_in": "double"}))

	s, a3 := Distribution(7, 11)
	fmt.Printf("distribution: hour 11 has %.0f %% empty amount_usd, alert %v\n", s*100, a3)

	fmt.Println("reconciliation, rows per block range match the chain:", Reconcile(10412, 10412))

	// the trap: only the job status is watched
	if err := runIndexerHour(); err != nil { // exit 0 even with 1 row
		alert("indexer job failed")
	}
	// hours 11 and 12 held 1 row each, nobody looked
	quiet := append(append([]int(nil), normal[:10]...), 1, 1)
	fmt.Printf("job status only: exit 0 = success, rows in hours 11-12: %d and %d, alerts raised: %d\n", quiet[10], quiet[11], alerts)
	// the fix: a monitor on the data
	if hour, ratio, cliff := Volume(quiet); cliff {
		alert(fmt.Sprintf("volume: hour %d at %.0f %% of usual", hour, ratio*100))
	}
}

// runIndexerHour stands for the indexer job: the decoder skipped the swaps
// without an error, so the job exits 0.
func runIndexerHour() error { return nil }

var alerts int

// alert pages the table owner.
func alert(msg string) {
	alerts++
	fmt.Println("ALERT to the table owner:", msg)
}
