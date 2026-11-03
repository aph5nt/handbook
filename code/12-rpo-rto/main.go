// Replays RPO and RTO: each option is judged against two numbers, then a
// restore nobody timed meets a corrupted replica.
package main

import "fmt"

// Target is what the business asked for, in minutes.
type Target struct {
	RPO, RTO float64
}

// Option is one way to protect the data, in minutes.
type Option struct {
	Name      string
	WorstLost float64
	Downtime  float64
	CopiesBad bool // a bad write reaches this copy too
}

// Judge compares an option with the target, one number at a time.
func Judge(o Option, t Target) string {
	lost := o.WorstLost > t.RPO
	late := o.Downtime > t.RTO
	switch {
	case lost && late:
		return "fails RPO and RTO"
	case lost:
		return "fails RPO"
	case late:
		return "fails RTO"
	}
	return "fits"
}

// LostSinceBackup is the gap to the last backup at the moment of the disaster.
func LostSinceBackup(disaster, lastBackup float64) float64 {
	return disaster - lastBackup
}

// CleanCopy finds the newest copy without the damage.
func CleanCopy(options []Option) string {
	for _, o := range options {
		if !o.CopiesBad {
			return o.Name
		}
	}
	return "none"
}

// RestoreMinutes grows with the backup: an old timing lies after the backup grows.
func RestoreMinutes(sizeGB, gbPerMinute float64) float64 {
	return sizeGB / gbPerMinute
}

// PointInTime replays the write log from the last snapshot up to just before the damage.
func PointInTime(damage, stopBefore float64) (restoreTo, lost float64) {
	return damage - stopBefore, stopBefore
}

func main() {
	ledger := Target{0, 15}
	for _, o := range []Option{
		{"nightly backup", 1440, 360, false},
		{"async replica", 0.1, 10, true},
		{"sync replica, rehearsed", 0, 5, true},
	} {
		fmt.Printf("ledger, %-24s lost %6.1f min, down %3.0f min: %s\n", o.Name, o.WorstLost, o.Downtime, Judge(o, ledger))
	}
	fmt.Println("disaster at 02:00, backup at 00:00 loses", LostSinceBackup(120, 0), "minutes")
	analytics := Target{1440, 1440}
	fmt.Println("analytics, nightly backup:", Judge(Option{"nightly backup", 1440, 360, false}, analytics))

	orders := Target{60, 120}
	fmt.Println("\norders database, RPO 60, RTO 120")
	copies := []Option{{"async replica", 0.1, 10, true}, {"last night's backup", 120, 0, false}}
	fmt.Println("after the bad migration, newest clean copy:", CleanCopy(copies))
	fmt.Printf("restore on paper: %.0f min. backup grew tenfold: %.0f min\n", RestoreMinutes(540, 10), RestoreMinutes(5400, 10))
	real := Option{"backup, measured live", 120, 570, false}
	fmt.Println("02:00 to 11:30:", real.Downtime, "min:", Judge(real, orders))
	split := Option{"backup split, drill 40 min", 120, 40, false}
	fmt.Println("restore drill:", Judge(split, orders))
	to, lost := PointInTime(120, 1)
	fmt.Printf("write log: restore to minute %.0f, lost %.0f min: %s\n", to, lost, Judge(Option{"hourly + log", lost, 40, false}, orders))
}
