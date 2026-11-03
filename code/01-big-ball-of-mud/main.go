// Big ball of mud: one shared Address type that every team grows.
// The way out: find a word with two meanings, cut one context out behind a translator.
package main

import "fmt"

// Field is one field of the shared Address and what each user means by it.
type Field struct {
	Name  string
	Users map[string]string // team -> meaning
}

// Mud is the shared models package: every team reads every field.
type Mud struct{ Fields map[string]*Field }

// Add is how each team grows the shared type for itself.
func (m *Mud) Add(team, name, meaning string) {
	f, ok := m.Fields[name]
	if !ok {
		f = &Field{Name: name, Users: map[string]string{}}
		m.Fields[name] = f
	}
	f.Users[team] = meaning
}

// Rename by one team breaks every other team that used the field.
func (m *Mud) Rename(team, name string) []string {
	var broken []string
	for t := range m.Fields[name].Users {
		if t != team {
			broken = append(broken, t)
		}
	}
	return broken
}

// Seam finds one word with two meanings: a place to cut.
func (m *Mud) Seam(name string) bool {
	seen := map[string]bool{}
	for _, meaning := range m.Fields[name].Users {
		seen[meaning] = true
	}
	return len(seen) > 1
}

// Cut moves one team out: its own type, fed by a translator (an ACL).
func (m *Mud) Cut(team string) {
	for _, f := range m.Fields {
		delete(f.Users, team)
	}
}

func main() {
	m := &Mud{Fields: map[string]*Field{}}
	for _, n := range []string{"hex", "first_block", "last_block", "is_contract", "nonce", "balance"} {
		m.Add("indexer", n, "chain fact")
	}
	m.Fields["balance"].Users["indexer"] = "tokens held"
	for team, n := range map[string]int{"risk": 9, "billing": 10, "reports": 8, "alerts": 7} {
		for k := 0; k < n; k++ {
			m.Add(team, fmt.Sprintf("%s_%d", team, k), team+" use")
		}
	}
	m.Add("billing", "balance", "money owed")
	m.Add("risk", "balance", "tokens held")
	fmt.Println("Address fields:", len(m.Fields))
	fmt.Println("billing renames balance, broken:", len(m.Rename("billing", "balance")), "teams")
	fmt.Println("balance has two meanings, a seam:", m.Seam("balance"))
	m.Cut("billing")                            // billing gets Wallet, fed through a translator
	wallet := &Mud{Fields: map[string]*Field{}} // billing's own type
	wallet.Add("billing", "owed", "money owed")
	fmt.Println("after the cut, billing renames Wallet.owed, broken:", len(wallet.Rename("billing", "owed")), "teams")
	fmt.Println("balance still a seam:", m.Seam("balance"))
}
