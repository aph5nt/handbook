// Avro versus Protobuf: how a reader finds the fields in the bytes.
//
// Not the real wire formats, but the same rule for each. "Avro" bytes are the
// values in writer order with no names, so a reader needs the writer's schema
// and matches by field name. "Protobuf" bytes pair each value with its tag
// number, so a reader matches by number. Standard library only. Deterministic.
package main

import "fmt"

// ---- Avro ----

type AField struct {
	Name    string
	Default string // "" means no default
	Aliases []string
}

// avroRead resolves the writer's values against the reader's schema by name.
func avroRead(writer []AField, values []string, reader []AField) (map[string]string, error) {
	byName := map[string]string{}
	for i, f := range writer {
		byName[f.Name] = values[i]
	}
	out := map[string]string{}
	for _, f := range reader {
		v, ok := byName[f.Name]
		for _, a := range f.Aliases {
			if w, found := byName[a]; found && !ok {
				v, ok = w, true
			}
		}
		if !ok {
			if f.Default == "" {
				return nil, fmt.Errorf("field %s: not in the writer, no default", f.Name)
			}
			v = f.Default
		}
		out[f.Name] = v
	}
	return out, nil
}

// ---- Protobuf ----

type Tagged struct {
	Tag  int
	Kind string
	Val  string
}

type PField struct {
	Tag        int
	Name, Kind string
}

// protoRead matches by tag number and keeps what it does not know.
func protoRead(bytes []Tagged, reader []PField) (map[string]string, []Tagged, error) {
	out := map[string]string{}
	var unknown []Tagged
	for _, t := range bytes {
		known := false
		for _, f := range reader {
			if f.Tag == t.Tag {
				if f.Kind != t.Kind {
					return nil, nil, fmt.Errorf("tag %d: %s read as %s (%s)", t.Tag, t.Kind, f.Kind, f.Name)
				}
				out[f.Name] = t.Val
				known = true
			}
		}
		if !known {
			unknown = append(unknown, t) // kept and written back unchanged
		}
	}
	return out, unknown, nil
}

func main() {
	fmt.Println("== Tab 1: Avro ==")
	w := []AField{{Name: "pool"}, {Name: "amount"}, {Name: "chain"}}
	vals := []string{"0x51", "12.5", "robinhood"}
	fmt.Println("bytes hold only:", vals)
	v1, _ := avroRead(w, vals, []AField{{Name: "pool"}, {Name: "amount"}})
	fmt.Println("older reader:", v1)
	v3, _ := avroRead(w, vals, []AField{{Name: "pool"}, {Name: "amount"}, {Name: "chain"}, {Name: "fee", Default: "0"}})
	fmt.Println("newer reader, fee from its default:", v3)
	_, err := avroRead(w, vals, []AField{{Name: "pool"}, {Name: "value"}})
	fmt.Println("rename:", err)
	fix, _ := avroRead(w, vals, []AField{{Name: "pool"}, {Name: "value", Aliases: []string{"amount"}}})
	fmt.Println("rename with an alias:", fix)

	fmt.Println("== Tab 2: Protobuf ==")
	b := []Tagged{{1, "string", "0x51"}, {2, "double", "12.5"}, {3, "string", "robinhood"}}
	r, unk, _ := protoRead(b, []PField{{1, "pool", "string"}, {2, "amount", "double"}})
	fmt.Println("older reader:", r, "unknown kept:", unk)
	r, _, _ = protoRead(b, []PField{{1, "pool", "string"}, {2, "value", "double"}})
	fmt.Println("a rename is free, the number is still 2:", r)
	_, _, err = protoRead(b, []PField{{1, "pool", "string"}, {2, "fee", "string"}})
	fmt.Println("number 2 reused for a string:", err)
	reserved := map[int]bool{2: true}
	fmt.Println("reserved 2: the compiler refuses reuse:", reserved[2])
}
