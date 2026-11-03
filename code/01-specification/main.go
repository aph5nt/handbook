// Specification: a business rule as an object. It checks one thing in memory,
// and the same object becomes the WHERE clause for many.
package main

import "fmt"

// Address is one thing to check.
type Address struct {
	Name     string
	Label    string
	DaysSeen int // days since last seen
	First    int // days since first seen
}

// Spec answers one question, and can write itself as SQL.
type Spec struct {
	IsSatisfiedBy func(Address) bool
	SQL           string
}

// And and Not build new rules with no new if statement.
func (s Spec) And(o Spec) Spec {
	return Spec{func(a Address) bool { return s.IsSatisfiedBy(a) && o.IsSatisfiedBy(a) }, s.SQL + " AND " + o.SQL}
}
func (s Spec) Not() Spec {
	return Spec{func(a Address) bool { return !s.IsSatisfiedBy(a) }, "NOT (" + s.SQL + ")"}
}

// The small rules, in the business language.
var IsExchange = Spec{func(a Address) bool { return a.Label == "exchange" }, "label = 'exchange'"}

func SeenInLast(days int) Spec {
	return Spec{func(a Address) bool { return a.DaysSeen <= days }, fmt.Sprintf("days_seen <= %d", days)}
}

// ActiveExchange is the one place that says what active means.
var ActiveExchange = IsExchange.And(SeenInLast(30))

// Repository turns the rule into a query: only matches come back.
func Repository(s Spec) string { return "SELECT address FROM addresses WHERE " + s.SQL }

// FindAllThenFilter is failure 2: right answer, every row loaded.
func FindAllThenFilter(all []Address, s Spec) (kept, loaded int) {
	for _, a := range all {
		loaded++
		if s.IsSatisfiedBy(a) {
			kept++
		}
	}
	return
}

func main() {
	all := []Address{{"A", "exchange", 3, 400}, {"B", "exchange", 90, 500}, {"C", "wallet", 1, 10}}
	for _, a := range all {
		fmt.Println(a.Name, "active exchange:", ActiveExchange.IsSatisfiedBy(a), "- not:", ActiveExchange.Not().IsSatisfiedBy(a))
	}
	fmt.Println(Repository(ActiveExchange))
	fmt.Println(Repository(ActiveExchange.Not()))
	// failure 1: the rule copied twice, and drifted
	alert := func(a Address) bool { return a.Label == "exchange" && a.DaysSeen <= 30 }
	report := func(a Address) bool { return a.Label == "exchange" && a.First <= 31 }
	drift := Address{"D", "exchange", 31, 31}
	fmt.Println("D, seen 31 days ago: alert", alert(drift), "- report", report(drift))
	kept, loaded := FindAllThenFilter(all, ActiveExchange)
	fmt.Println("filter in memory: kept", kept, "of", loaded, "loaded")
}
