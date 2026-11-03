// Hot, warm and cold tiers: a nightly Go job decides each day partition's
// tier by its age and prints the StarRocks statement it would run.
package main

import (
	"fmt"
	"time"
)

const (
	hotDays  = 7   // younger than this stays on SSD
	warmDays = 180 // younger than this sits on hard disk, older goes cold
	ssdSlots = 10  // partitions the SSD can hold in this story
)

type Partition struct {
	Name string
	Day  time.Time
	Tier string // "hot", "warm" or "cold"
}

// Rule is a table's tier rule. A table with no rule is never moved.
type Rule struct{ HotDays, WarmDays int }

// TierFor decides a partition's tier from its age in days.
func TierFor(day, today time.Time, r Rule) string {
	age := int(today.Sub(day).Hours() / 24)
	switch {
	case age < r.HotDays:
		return "hot"
	case age < r.WarmDays:
		return "warm"
	default:
		return "cold"
	}
}

// RunTiering moves every partition whose tier changed, for tables with a rule.
func RunTiering(table string, parts []Partition, rules map[string]Rule, today time.Time) (moved int) {
	r, ok := rules[table]
	if !ok {
		return 0 // no rule: the job skips this table
	}
	for k, p := range parts {
		want := TierFor(p.Day, today, r)
		if want == p.Tier {
			continue
		}
		switch want {
		case "warm":
			fmt.Printf("  ALTER TABLE %s MODIFY PARTITION %s SET (\"storage_medium\" = \"HDD\");\n", table, p.Name)
		case "cold":
			fmt.Printf("  INSERT INTO FILES(\"path\" = \"s3://cold/%s/%s/\", \"format\" = \"parquet\") SELECT * FROM %s PARTITION (%s);\n", table, p.Name, table, p.Name)
			fmt.Printf("  ALTER TABLE %s DROP PARTITION %s; -- now read through the external catalog\n", table, p.Name)
		}
		parts[k].Tier = want
		moved++
	}
	return moved
}

// Read lists the partitions a day filter keeps. No filter keeps all of them.
func Read(parts []Partition, from, to time.Time, filtered bool) (tiers map[string]int) {
	tiers = map[string]int{}
	for _, p := range parts {
		if filtered && (p.Day.Before(from) || p.Day.After(to)) {
			continue // pruned by the day filter
		}
		tiers[p.Tier]++
	}
	return tiers
}

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func main() {
	today := day(2026, 11, 2)
	names := []string{"p20261102", "p20261101", "p20261031", "p20261030", "p202609", "p202608", "p2025", "p2024"}
	days := []time.Time{day(2026, 11, 2), day(2026, 11, 1), day(2026, 10, 31), day(2026, 10, 30),
		day(2026, 9, 15), day(2026, 8, 15), day(2025, 6, 1), day(2024, 6, 1)}
	var parts []Partition
	for k := range names {
		parts = append(parts, Partition{names[k], days[k], "hot"}) // every day lands on SSD
	}
	rules := map[string]Rule{"transfers": {hotDays, warmDays}}
	fmt.Println("nightly tiering job, transfers:")
	moved := RunTiering("transfers", parts, rules, today)
	count := map[string]int{}
	for _, p := range parts {
		count[p.Tier]++
	}
	fmt.Printf("moved %d partitions: hot %d, warm %d, cold %d\n", moved, count["hot"], count["warm"], count["cold"])
	fmt.Println("this week reads:", Read(parts, day(2026, 10, 27), today, true))
	fmt.Println("audit of 2024 reads:", Read(parts, day(2024, 1, 1), day(2024, 12, 31), true))
	fmt.Println("no day filter reads:", Read(parts, today, today, false))

	// swaps_v2 was created without a rule: the job skips it
	var v2 []Partition
	for k := range names {
		v2 = append(v2, Partition{names[k], days[k], "hot"})
	}
	moved = RunTiering("swaps_v2", v2, rules, today)
	otherHot := 2 // hot days of the tables that do have a rule
	fmt.Printf("swaps_v2, no rule: moved %d, on SSD %d, SSD used %d of %d", moved, len(v2), len(v2)+otherHot, ssdSlots)
	if len(v2)+otherHot >= ssdSlots {
		fmt.Print(": full, new day partitions cannot be written")
	}
	fmt.Println()
	rules["swaps_v2"] = Rule{hotDays, warmDays} // the fix: a rule at create time
	fmt.Println("swaps_v2 with a rule:")
	RunTiering("swaps_v2", v2, rules, today)
	hot := 0
	for _, p := range v2 {
		if p.Tier == "hot" {
			hot++
		}
	}
	fmt.Printf("swaps_v2 on SSD %d\n", hot)
}
