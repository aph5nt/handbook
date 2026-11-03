// Structured output: outputSchema is the promise, structuredContent the typed
// value, and the host validates before code reads it. Run: go run .
package main

import (
	"encoding/json"
	"fmt"
)

// A small JSON Schema: property name -> "string" or "number".
var outputSchema = map[string]string{"address": "string", "balance": "number", "usd": "number"}

type Result struct {
	StructuredContent map[string]any   `json:"structuredContent"`
	Content           []map[string]any `json:"content"`
}

func getBalance(asString bool) Result {
	sc := map[string]any{"address": "0x18..cb24", "balance": 12.5, "usd": 183.0}
	if asString { // the server team's silent change
		sc["balance"] = "12.5"
	}
	b, _ := json.Marshal(sc)
	return Result{sc, []map[string]any{{"type": "text", "text": string(b)}}} // same JSON for old hosts
}

// validate checks each declared field has the declared JSON type.
func validate(sc map[string]any) error {
	for k, want := range outputSchema {
		got := ""
		switch sc[k].(type) {
		case string:
			got = "string"
		case float64:
			got = "number"
		}
		if got != want {
			return fmt.Errorf("%s: want %s, got %q", k, want, fmt.Sprint(sc[k]))
		}
	}
	return nil
}

// A script that adds balances with no check. Text glue is what you get from strings.
func addUnchecked(total string, sc map[string]any) string {
	return total + fmt.Sprint(sc["balance"])
}

func main() {
	b, _ := json.Marshal(outputSchema)
	fmt.Println("0 outputSchema:", string(b))
	r := getBalance(false)
	w, _ := json.Marshal(r)
	fmt.Println("1 result:", string(w))
	fmt.Println("2 validate:", validate(r.StructuredContent))
	total := 0.0
	total += r.StructuredContent["balance"].(float64)
	fmt.Println("3 total =", total)

	bad := getBalance(true)
	fmt.Println("4 unchecked total:", addUnchecked(addUnchecked("0", bad.StructuredContent), bad.StructuredContent))
	fmt.Println("5 validate:", validate(bad.StructuredContent))
}
