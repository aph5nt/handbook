// Backward and forward compatibility: who may upgrade first.
//
// One rule, canRead(reader, writer), gives all three modes:
//
//	BACKWARD: the new schema can read old data  -> consumers upgrade first
//	FORWARD:  the old schema can read new data  -> producer upgrades first
//	FULL:     both                              -> any order, any replay
//
// Plain structs stand in for the schemas. Deterministic.
package main

import "fmt"

type Field struct {
	Name, Type string
	HasDefault bool
}
type Schema []Field

func canRead(reader, writer Schema) error {
	for _, f := range reader {
		found := false
		for _, w := range writer {
			if w.Name == f.Name {
				found = true
				if w.Type != f.Type {
					return fmt.Errorf("%s: %s read as %s", f.Name, w.Type, f.Type)
				}
			}
		}
		if !found && !f.HasDefault {
			return fmt.Errorf("%s: not in the data, no default", f.Name)
		}
	}
	return nil
}

func check(mode string, old, new Schema) error {
	switch mode {
	case "BACKWARD":
		return canRead(new, old)
	case "FORWARD":
		return canRead(old, new)
	}
	if err := canRead(new, old); err != nil {
		return err
	}
	return canRead(old, new)
}

func main() {
	pool, amount := Field{"pool", "string", false}, Field{"amount", "double", false}

	fmt.Println("== Tab 1: BACKWARD ==")
	v1 := Schema{pool, amount, {"fee", "double", false}}
	v2 := Schema{pool, amount} // removes fee
	fmt.Printf("registry, v2 removes fee: BACKWARD err=%v\n", check("BACKWARD", v1, v2))
	fmt.Printf("consumer on v2 reads v1 data: err=%v\n", canRead(v2, v1))
	fmt.Printf("wrong order, consumer on v1 reads v2 data: err=%v\n", canRead(v1, v2))

	fmt.Println("== Tab 2: FORWARD ==")
	f1 := Schema{pool, amount}
	f2 := Schema{pool, amount, {"chain", "string", false}} // adds chain, no default
	fmt.Printf("registry, v2 adds chain with no default: FORWARD err=%v\n", check("FORWARD", f1, f2))
	fmt.Printf("consumer on v1 reads v2 data: err=%v\n", canRead(f1, f2))
	fmt.Printf("replay, consumer on v2 reads v1 data: err=%v\n", canRead(f2, f1))

	fmt.Println("== Tab 3: FULL ==")
	g2 := Schema{pool, amount, {"chain", "string", true}} // adds chain with a default
	fmt.Printf("add chain with a default: FULL err=%v\n", check("FULL", f1, g2))
	g3 := Schema{pool, {"amount", "string", false}, {"chain", "string", true}}
	fmt.Printf("amount to text: FULL err=%v\n", check("FULL", g2, g3))
	fmt.Println("no rule allows it: new topic swaps.v3, write to both, move readers, stop the old")
}
