// One contract file feeds a mock, a typed client, request validation and a
// docs page. Replays the lesson 13-openapi. The file is a small Go value in
// the shape of an OpenAPI document, so the program needs only the standard
// library. Real tools read openapi.yaml in the same way.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
)

// Spec is the contract for GET /transfers.
type Spec struct {
	Path   string
	Params map[string]string // query parameter -> pattern it must match
	Fields map[string]string // response field -> JSON type
}

var openapi = Spec{
	Path:   "/transfers",
	Params: map[string]string{"day": `^\d{4}-\d{2}-\d{2}$`},
	Fields: map[string]string{"id": "string", "amount": "number"},
}

// mock answers from the file alone, before the backend exists.
func mock(s Spec) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		row := map[string]any{}
		for f, t := range s.Fields {
			if t == "number" {
				row[f] = 12.5
			} else {
				row[f] = "example"
			}
		}
		json.NewEncoder(w).Encode([]map[string]any{row})
	})
}

// validate checks each request against the file before the handler runs.
func validate(s Spec, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for name, pattern := range s.Params {
			v := r.URL.Query().Get(name)
			if v != "" && !regexp.MustCompile(pattern).MatchString(v) {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"type": "invalid-params", "field": name})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// Transfer is the typed client's row, generated from the file.
type Transfer struct {
	ID     string  `json:"id"`
	Amount float64 `json:"amount"`
}

// getTransfers is the generated client call: not a hand-written URL.
func getTransfers(base, day string) ([]Transfer, int, error) {
	res, err := http.Get(base + openapi.Path + "?day=" + day)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	var rows []Transfer
	if res.StatusCode != 200 {
		return nil, res.StatusCode, nil
	}
	err = json.NewDecoder(res.Body).Decode(&rows)
	return rows, res.StatusCode, err
}

// docs renders the page from the file.
func docs(s Spec) string {
	var names []string
	for f, t := range s.Fields {
		names = append(names, f+": "+t)
	}
	sort.Strings(names)
	return fmt.Sprint("GET ", s.Path, " fields ", names)
}

// lint compares a changed file with the file on main: a lost field breaks.
func lint(main, changed Spec) []string {
	var fails []string
	for f := range main.Fields {
		if _, ok := changed.Fields[f]; !ok {
			fails = append(fails, "field removed: "+f)
		}
	}
	return fails
}

// conforms checks a real answer against the file: the drift test.
func conforms(s Spec, base string) []string {
	res, err := http.Get(base + s.Path)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	var rows []map[string]any
	json.NewDecoder(res.Body).Decode(&rows)
	var fails []string
	for f, want := range s.Fields {
		got := "string"
		if _, isNumber := rows[0][f].(float64); isNumber {
			got = "number"
		}
		if got != want {
			fails = append(fails, f+" is "+got+", the file says "+want)
		}
	}
	return fails
}

// server is the hand-written backend. It may drift from the file.
func server(amount any) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{{"id": "t1", "amount": amount}})
	})
}

func main() {
	fmt.Println("--- one file, many tools")
	m := httptest.NewServer(mock(openapi))
	defer m.Close()
	rows, code, _ := getTransfers(m.URL, "2026-10-01")
	fmt.Println("mock, no backend yet:", code, rows)
	s := httptest.NewServer(validate(openapi, server(12.5)))
	defer s.Close()
	rows, code, _ = getTransfers(s.URL, "2026-10-01")
	fmt.Println("typed client:", code, rows[0].Amount)
	_, code, _ = getTransfers(s.URL, "yesterday")
	fmt.Println("day=yesterday:", code, "field day named")
	fmt.Println("docs:", docs(openapi))
	renamed := Spec{Path: "/transfers", Fields: map[string]string{"id": "string", "value": "number"}}
	fmt.Println("lint on rename:", lint(openapi, renamed))

	fmt.Println("--- the server drifts from the file")
	drift := httptest.NewServer(server("12.5"))
	defer drift.Close()
	fmt.Println("lint, file unchanged:", lint(openapi, openapi))
	_, _, err := getTransfers(drift.URL, "2026-10-01")
	fmt.Println("generated client:", err)
	fmt.Println("conformance test:", conforms(openapi, drift.URL))
	fixed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{{"id": "t1", "amount": 12.5, "amount_text": "12.5"}})
	}))
	defer fixed.Close()
	fmt.Println("with amount_text added:", conforms(openapi, fixed.URL))
}
