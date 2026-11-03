// A contract test starts the real server over stdio and checks what a host
// sees: the exact tool names, descriptions, hints, skill doors and golden
// results. A unit test calls the function and misses a rename. Run: go run .
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Annotations map[string]bool `json:"annotations"`
}

// The proxy after the rename. The description was not updated.
var tools = []Tool{
	{"dex_lookup", "Find an exchange by name. Call meta_dex_lookup first.", map[string]bool{"readOnlyHint": true}},
	{"graph_query", "Read the graph. Anchor on one address.", map[string]bool{"readOnlyHint": true}},
}

func lookup(name string) string { return "Aerodrome: 0x8f..2c11" } // what unit tests call

func serve(in io.Reader, out io.Writer) {
	sc := bufio.NewScanner(in)
	for sc.Scan() {
		var m struct {
			ID     int
			Method string
		}
		json.Unmarshal(sc.Bytes(), &m)
		switch m.Method {
		case "initialize":
			fmt.Fprintf(out, `{"jsonrpc":"2.0","id":%d,"result":{}}`+"\n", m.ID)
		case "tools/list":
			b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": m.ID, "result": map[string]any{"tools": tools}})
			fmt.Fprintln(out, string(b))
		case "tools/call":
			fmt.Fprintf(out, `{"jsonrpc":"2.0","id":%d,"result":{"content":[{"type":"text","text":%q}]}}`+"\n", m.ID, lookup("aero"))
		}
	}
}

func contractTest(expected []string, skills []string, golden string) []string {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	go serve(inR, outW) // the real server, spoken to like a host
	rd := bufio.NewScanner(outR)
	ask := func(l string) []byte { fmt.Fprintln(inW, l); rd.Scan(); return rd.Bytes() }
	ask(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	fmt.Fprintln(inW, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	var lr struct{ Result struct{ Tools []Tool } }
	json.Unmarshal(ask(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`), &lr)

	var fails, names []string
	have := map[string]bool{}
	for _, t := range lr.Result.Tools {
		names = append(names, t.Name)
		have[t.Name] = true
	}
	sort.Strings(names)
	if strings.Join(names, ",") != strings.Join(expected, ",") {
		fails = append(fails, fmt.Sprintf("names %v, want %v", names, expected))
	}
	ref := regexp.MustCompile(`\b[a-z]+_[a-z_]+\b`) // a tool name inside a description
	for _, t := range lr.Result.Tools {
		for _, n := range ref.FindAllString(t.Description, -1) {
			if !have[n] {
				fails = append(fails, t.Name+" description names missing tool "+n)
			}
		}
		if !t.Annotations["readOnlyHint"] {
			fails = append(fails, t.Name+" has no readOnlyHint")
		}
	}
	for _, s := range skills { // every skill needs a tool-shaped door
		if !have[s] {
			fails = append(fails, "skill "+s+" has no door")
		}
	}
	if got := ask(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"meta_dex_lookup"}}`); !strings.Contains(string(got), golden) {
		fails = append(fails, "golden mismatch")
	}
	return fails
}

func main() {
	fmt.Println("unit 2: lookup(\"aero\") =", lookup("aero"), "-> PASS")
	expected := []string{"graph_query", "meta_dex_lookup"}
	for _, f := range contractTest(expected, []string{"meta_dex_lookup"}, "Aerodrome: 0x8f..2c11") {
		fmt.Println("wire FAIL:", f)
	}
	tools[0] = Tool{"meta_dex_lookup", "Find an exchange by name. Use its address in graph_query.", map[string]bool{"readOnlyHint": true}}
	fmt.Println("wire after the fix, failures:", len(contractTest(expected, []string{"meta_dex_lookup"}, "Aerodrome: 0x8f..2c11")))
}
