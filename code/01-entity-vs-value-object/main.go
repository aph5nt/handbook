// Entity: identity survives change. Value object: just its values, so it is
// replaced, never changed, and it is born valid.
package main

import (
	"errors"
	"fmt"
)

// Customer is an entity: compared by ID, its fields change in place.
type Customer struct {
	ID, Name, City string
}

func (c Customer) Same(o Customer) bool { return c.ID == o.ID }

// Money is a value object. Its fields are private, and NewMoney checks them.
type Money struct {
	cents    int
	currency string
}

// NewMoney is the only door: a Money that exists is always valid.
func NewMoney(cents int, cur string) (Money, error) {
	if cents < 0 {
		return Money{}, errors.New("money cannot be negative")
	}
	return Money{cents, cur}, nil
}

// Minus makes a new Money. The old one stays what it was.
func (m Money) Minus(cents int) (Money, error) { return NewMoney(m.cents-cents, m.currency) }

func (m Money) String() string { return fmt.Sprintf("%d %s", m.cents/100, m.currency) }

// Order holds a Money by value: nobody can change it through another order.
type Order struct {
	ID    int
	Total Money
}

// MutableMoney is the failure: two orders can point at one changeable amount.
type MutableMoney struct{ Cents int }
type BadOrder struct {
	ID    int
	Total *MutableMoney
}

func main() {
	a := Customer{"c-17", "Anna Nowak", "Warsaw"}
	b := Customer{"c-42", "Anna Nowak", "Gdansk"}
	fmt.Println("same name, same customer?", a.Same(b))
	moved := a
	moved.City = "Krakow"
	fmt.Println("c-17 moved, same customer?", a.Same(moved))
	x, _ := NewMoney(1000, "EUR")
	y, _ := NewMoney(1000, "EUR")
	fmt.Println("10 EUR == 10 EUR:", x == y)
	z, _ := x.Minus(200)
	fmt.Println("discount:", z, "old still", x)
	_, err := NewMoney(-500, "EUR")
	fmt.Println("minus 5 EUR:", err)

	shared := &MutableMoney{1000}
	o1, o2 := BadOrder{1001, shared}, BadOrder{1002, shared}
	o1.Total.Cents -= 200
	fmt.Println("shared, changed in place: 1001 =", o1.Total.Cents/100, "1002 =", o2.Total.Cents/100)

	g1, g2 := Order{1001, x}, Order{1002, x}
	g1.Total, _ = g1.Total.Minus(200)
	fmt.Println("immutable: 1001 =", g1.Total, "1002 =", g2.Total)
}
