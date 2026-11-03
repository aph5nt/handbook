// Schema registry: every schema is checked before a message can use it.
//
// A Go struct stands in for the registry service, with a BACKWARD check:
// the new schema as reader must be able to read the latest stored schema.
// The JSON half uses encoding/json, with no check at all. Deterministic.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
)

type Field struct {
	Name, Type string
	Default    string // "" means no default
}
type Schema []Field

func (s Schema) find(name string) (Field, bool) {
	for _, f := range s {
		if f.Name == name {
			return f, true
		}
	}
	return Field{}, false
}

// canRead: every reader field is in the writer with the same type, or has a default.
func canRead(reader, writer Schema) error {
	for _, f := range reader {
		w, ok := writer.find(f.Name)
		if !ok && f.Default == "" {
			return fmt.Errorf("field %s: missing, no default", f.Name)
		}
		if ok && w.Type != f.Type {
			return fmt.Errorf("field %s: %s changed to %s", f.Name, w.Type, f.Type)
		}
	}
	return nil
}

type Registry struct {
	next    int
	schemas map[int]Schema
	latest  int
}

// Register stores a schema and returns its id, or refuses it.
func (r *Registry) Register(s Schema) (int, error) {
	if r.latest != 0 {
		if err := canRead(s, r.schemas[r.latest]); err != nil {
			return 0, err
		}
	}
	id := r.next
	r.next++
	r.schemas[id] = s
	r.latest = id
	return id, nil
}

func main() {
	fmt.Println("== Tab 1: with a registry ==")
	r := &Registry{next: 41, schemas: map[int]Schema{}}
	v1 := Schema{{"pool", "string", ""}, {"amount", "double", ""}}
	id, _ := r.Register(v1)
	fmt.Printf("v1 registered as id %d, each message carries only %d\n", id, id)
	v2 := append(Schema{}, v1...)
	v2 = append(v2, Field{"chain", "string", "robinhood"})
	id, err := r.Register(v2)
	fmt.Printf("v2 adds chain with a default: id %d, err=%v\n", id, err)
	fmt.Printf("old reader of v2 data: %v. new reader of v1 data: %v\n", canRead(v1, v2), canRead(v2, v1))
	v3 := Schema{{"pool", "string", ""}, {"amount", "string", ""}, {"chain", "string", "robinhood"}}
	_, err = r.Register(v3)
	fmt.Printf("v3 changes amount to text: refused: %v\n", err)

	fmt.Println("== Tab 2: JSON with no schema ==")
	decode := func(b []byte) (float64, error) {
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			return 0, err
		}
		a, ok := m["amount"].(float64)
		if !ok {
			return 0, errors.New("amount: not found")
		}
		return a, nil
	}
	a, err := decode([]byte(`{"amount":12.5}`))
	fmt.Printf("old message: amount %v, err=%v\n", a, err)
	_, err = decode([]byte(`{"value":"12.5"}`))
	fmt.Printf("renamed message, accepted by the broker: %v\n", err)
	renamed := Schema{{"pool", "string", ""}, {"value", "string", ""}, {"chain", "string", "robinhood"}}
	_, err = r.Register(renamed) // field value: missing, no default
	fmt.Printf("the same rename against the registry: refused: %v\n", err)
}
