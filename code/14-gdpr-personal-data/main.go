// GDPR (General Data Protection Regulation) in code: a field declaration
// that must name a lawful basis and a retention, one path by person id for
// every right, and a data map that says how each copy is erased.
package main

import (
	"fmt"
	"time"
)

type field struct {
	name, lawfulBasis string
	retentionDays     int
}

// review refuses a personal field with no basis or no retention.
func review(f field) error {
	if f.lawfulBasis == "" {
		return fmt.Errorf("%s: no lawful basis", f.name)
	}
	if f.retentionDays == 0 {
		return fmt.Errorf("%s: no retention", f.name)
	}
	return nil
}

type copyOfData struct {
	place, owner string
	erase        func(person string) string // nil means nobody knows how
}

var labels = map[string]string{"p-42": "Ada Lovelace"}

// exportFor is the one path by person id: access and export use it.
func exportFor(person string) map[string]string {
	return map[string]string{"label": labels[person]}
}

// eraseAll walks the data map. A copy with no erase step is a finding.
func eraseAll(person string, dataMap []copyOfData) (done, missing []string) {
	for _, c := range dataMap {
		if c.erase == nil {
			missing = append(missing, c.place)
			continue
		}
		done = append(done, c.erase(person))
	}
	return done, missing
}

// breachDeadline is the 72 hours from finding the breach.
func breachDeadline(found time.Time) time.Time { return found.Add(72 * time.Hour) }

func main() {
	fmt.Println("Tab 1: design")
	fmt.Println("  owner name, no basis ->", review(field{"owner_name", "", 0}))
	fmt.Println("  owner name, no retention ->", review(field{"owner_name", "legal duty", 0}))
	fmt.Println("  owner name, both ->", review(field{"owner_name", "legal duty", 365}))
	fmt.Println("  export for p-42 ->", exportFor("p-42"))
	found := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	fmt.Println("  breach found", found.Format("Jan 2 15:04"), "-> tell the regulator by", breachDeadline(found).Format("Jan 2 15:04"))

	fmt.Println("Tab 2: erase")
	delLabel := func(p string) string { delete(labels, p); return "label store: rows deleted" }
	before := []copyOfData{{"label store", "support", delLabel}, {"debug logs", "", nil}, {"search index", "", nil}, {"CSV exports", "", nil}, {"backups", "", nil}}
	done, missing := eraseAll("p-42", before)
	fmt.Println("  no data map: done", done, " nobody knows:", missing)
	after := []copyOfData{
		{"label store", "support", delLabel},
		{"debug logs", "platform", func(string) string { return "debug logs: hold ids only, nothing to erase" }},
		{"search index", "search", func(string) string { return "search index: reindexed" }},
		{"CSV exports", "analysts", func(string) string { return "CSV exports: expired after 30 days" }},
		{"event log and backups", "data", func(string) string { return "event log and backups: key deleted (crypto-shredding)" }},
	}
	done, missing = eraseAll("p-42", after)
	fmt.Println("  data map:", len(done), "steps done, missing:", len(missing))
	for _, d := range done {
		fmt.Println("   ", d)
	}
}
