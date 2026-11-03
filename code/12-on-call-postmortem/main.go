// Replays the on-call night: a page that escalates by timer, a commander who
// declares, a runbook that rolls back, and a postmortem whose actions are
// checked. Times are minutes after midnight.
package main

import (
	"fmt"
	"strings"
)

const ackTimeout = 5 // minutes before the page moves on

// Person is one name on the rotation.
type Person struct {
	Name  string
	AckIn int // minutes to acknowledge, -1 never
}

// Escalate walks the rotation until somebody acknowledges.
func Escalate(start int, rotation []Person) (who string, at int) {
	t := start
	for _, p := range rotation {
		if p.AckIn >= 0 && p.AckIn <= ackTimeout {
			return p.Name, t + p.AckIn
		}
		t += ackTimeout
	}
	return "", t
}

// Declare is the rule for an incident: users are affected.
func Declare(usersAffected bool) string {
	if usersAffected {
		return "incident: take command"
	}
	return "ticket"
}

// Runbook is the first safe step the alert links to.
func Runbook(hasRunbook, releasedToday bool) (string, int) {
	if !hasRunbook {
		return "no runbook: search by hand", 260
	}
	if releasedToday {
		return "roll back the release", 15
	}
	return "read the next runbook step", 30
}

// Action is one postmortem item.
type Action struct {
	Text, Owner, Due string
}

// Review refuses a postmortem item that will not change the system.
func Review(a Action) string {
	if strings.Contains(a.Text, "careful") {
		return "refused: not an action"
	}
	if a.Owner == "" || a.Due == "" {
		return "refused: no owner or date"
	}
	return "tracked"
}

// Clock prints minutes after midnight.
func Clock(m int) string {
	return fmt.Sprintf("%02d:%02d", m/60, m%60)
}

func main() {
	page := 3*60 + 10
	rotation := []Person{{"primary, on a train", -1}, {"secondary", 3}}
	fmt.Println(Clock(page), "page primary")
	who, at := Escalate(page, rotation)
	fmt.Println(Clock(page+ackTimeout), "no ack after", ackTimeout, "minutes: page secondary")
	fmt.Println(Clock(at), who, "acknowledges:", Declare(true))
	rb, fix := Runbook(true, true)
	roll := at + 7
	fmt.Println(Clock(roll), "runbook:", rb)
	fmt.Println(Clock(roll+fix), "data caught up, incident closed,", roll+fix-page, "minutes")
	for _, a := range []Action{
		{"release check", "Ana", "Nov 9"},
		{"faster rollback", "Ben", "Nov 16"},
		{"runbook update", "Chi", "Nov 9"},
		{"alert on disk", "Dan", "Nov 9"},
	} {
		fmt.Println("  action", a.Text, "->", Review(a))
	}

	fmt.Println("\ntab 2")
	fmt.Println("  action be more careful ->", Review(Action{"be more careful", "engineer X", "never"}))
	hero := []Person{{"primary, no notes", -1}, {"hero, on holiday", -1}}
	who, at = Escalate(page, hero)
	fmt.Printf("  nobody acknowledges (%q) by %s\n", who, Clock(at))
	rb, fix = Runbook(false, true)
	fmt.Println(" ", rb, ": data back at", Clock(page+fix))
}
