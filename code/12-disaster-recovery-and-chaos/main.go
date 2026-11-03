// Replays the disaster: a failover nobody tried, walked one dependency at a
// time, then the same failover as a drill with an abort switch.
package main

import "fmt"

// Dep is one thing the failover needs in region 2.
type Dep struct {
	Name    string
	Ready   bool // it works without a person
	Minutes int  // time when ready
	ByHand  int  // time when a person must rebuild it
}

// Result is what a failover cost.
type Result struct {
	Down    int
	LostSec int
	Gap     string
}

// Failover walks the dependencies in order. A drill stops at the first gap.
func Failover(deps []Dep, lagSec int, drill bool) Result {
	r := Result{LostSec: lagSec}
	for _, d := range deps {
		if d.Ready {
			r.Down += d.Minutes
			continue
		}
		if drill {
			r.Gap = d.Name
			return r
		}
		r.Down += d.ByHand
	}
	return r
}

// Hypothesis compares a drill with the numbers the business gave.
func Hypothesis(r Result, rtoMin, rpoSec int) string {
	if r.Gap != "" {
		return "abort: gap in " + r.Gap
	}
	if r.Down > rtoMin || r.LostSec > rpoSec {
		return "false: over target"
	}
	return "holds"
}

// Backup is the copy of last resort.
type Backup struct {
	RestoreMin int
	NewestOK   bool
}

// RestoreDrill measures the backup instead of hoping.
func RestoreDrill(b Backup) string {
	if !b.NewestOK {
		return fmt.Sprintf("%d minutes, newest file corrupt: a hope, not a backup", b.RestoreMin)
	}
	return fmt.Sprintf("%d minutes, last file readable", b.RestoreMin)
}

func clock(start, min int) string {
	m := start + min
	return fmt.Sprintf("%02d:%02d", m/60, m%60)
}

func main() {
	night := []Dep{
		{"DNS", false, 1, 40},
		{"secrets", false, 1, 170},
		{"promote replica", true, 10, 0},
	}
	r := Failover(night, 240, false)
	fmt.Printf("night: serving at %s, down %d min, lost %d s of writes\n", clock(120, r.Down), r.Down, r.LostSec)
	fmt.Println("backup:", RestoreDrill(Backup{540, false}))

	const rto, rpo = 30, 300
	fmt.Printf("\ndrill: targets RTO %d min, RPO %d s\n", rto, rpo)
	first := []Dep{{"DNS", true, 1, 0}, {"secrets", false, 1, 170}, {"promote replica", true, 2, 0}}
	r = Failover(first, 3, true)
	fmt.Println("first drill at 11:00:", Hypothesis(r, rto, rpo), "at", clock(11*60, 4), "users back in 1 minute")
	fixed := []Dep{{"DNS", true, 1, 0}, {"secrets", true, 1, 0}, {"promote replica", true, 2, 0}}
	r = Failover(fixed, 3, true)
	fmt.Printf("next drill: down %d min, lost %d s: %s\n", r.Down, r.LostSec, Hypothesis(r, rto, rpo))
	fmt.Println("restore drill:", RestoreDrill(Backup{40, true}))
}
