// Replays the ADR, C4 and fitness functions lesson. Three fitness functions
// run in the build: no semicolon in a Mermaid label, the domain package never
// imports the infrastructure package, and every ADR names its rejected
// options. A pull request merges only when all three pass.
package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// A label is the text inside [..], (..), {..} or |..| on a Mermaid line.
var label = regexp.MustCompile(`\[[^\]]*\]|\([^)]*\)|\{[^}]*\}|\|[^|]*\|`)

// mermaidSemicolons fails any Mermaid label that holds a semicolon,
// because the renderer cuts the line there.
func mermaidSemicolons(name, md string) []string {
	var bad []string
	in := false
	for i, line := range strings.Split(md, "\n") {
		switch {
		case strings.HasPrefix(line, "```mermaid"):
			in = true
		case strings.HasPrefix(line, "```"):
			in = false
		case in:
			for _, l := range label.FindAllString(line, -1) {
				if strings.Contains(l, ";") {
					bad = append(bad, fmt.Sprintf("%s:%d: label %s has a ;", name, i+1, l))
				}
			}
		}
	}
	return bad
}

// domainImportsInfra parses the imports of a Go file in the domain package
// and fails any import of the infrastructure package.
func domainImportsInfra(name, src string) []string {
	if !strings.HasPrefix(name, "domain/") {
		return nil
	}
	f, err := parser.ParseFile(token.NewFileSet(), name, src, parser.ImportsOnly)
	if err != nil {
		return []string{name + ": " + err.Error()}
	}
	var bad []string
	for _, imp := range f.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		if strings.Contains(path, "/infra") {
			bad = append(bad, fmt.Sprintf("%s: domain imports %s", name, path))
		}
	}
	return bad
}

// adrHasRejected fails an ADR with no rejected options written down.
func adrHasRejected(name, md string) []string {
	if strings.HasPrefix(name, "docs/adr/") && !strings.Contains(md, "## Rejected") {
		return []string{name + ": no rejected options"}
	}
	return nil
}

// build runs every fitness function over every file of the pull request.
func build(files map[string]string, checks []func(string, string) []string) []string {
	var bad []string
	for _, name := range sortedNames(files) {
		for _, check := range checks {
			bad = append(bad, check(name, files[name])...)
		}
	}
	return bad
}

func sortedNames(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func verdict(pr string, bad []string) {
	if len(bad) == 0 {
		fmt.Printf("  %s: 0 failures -> merged\n", pr)
		return
	}
	fmt.Printf("  %s: %d failure(s) -> blocked\n", pr, len(bad))
	for _, b := range bad {
		fmt.Println("    " + b)
	}
}

const adr12 = `# ADR 12: the graph is a projection of SQL
Status: accepted
## Decision
The graph is rebuilt from the warehouse. It is never the source of truth.
## Rejected
The graph as the source of truth: two stores of truth drift apart.`

const adr12NoRejected = `# ADR 12: the graph is a projection of SQL
Status: accepted
## Decision
The graph is rebuilt from the warehouse. It is never the source of truth.`

const c4Bad = "```mermaid\nflowchart LR\n  I[Indexer; fetch and decode] --> W[(Warehouse)]\n```"
const c4Good = "```mermaid\nflowchart LR\n  I[Indexer: fetch and decode] --> W[(Warehouse)]\n```"

const domainBad = `package domain

import "example.com/indexer/infra/starrocks"

var _ = starrocks.Open`

func main() {
	all := []func(string, string) []string{mermaidSemicolons, domainImportsInfra, adrHasRejected}

	fmt.Println("tab 1: the decision goes into the build")
	fmt.Printf("  ADR 12 check: %d failures\n", len(adrHasRejected("docs/adr/0012-graph-is-a-projection.md", adr12)))
	verdict("PR adds a diagram with a ; in a label", build(map[string]string{"docs/arch.md": c4Bad}, all))
	verdict("PR with the label fixed", build(map[string]string{"docs/arch.md": c4Good}, all))
	fmt.Println("  ADR 31 supersedes ADR 12: ADR 12 is kept as written, decisions recorded: 2")

	fmt.Println("tab 2: rules on a wiki")
	none := []func(string, string) []string{} // the wiki: the build runs nothing
	fmt.Printf("  rules in the build: %d\n", len(none))
	verdict("PR adds domain -> infra import", build(map[string]string{"domain/swap.go": domainBad}, none))
	fmt.Println("  the fix: the rules move into the build")
	fmt.Printf("  rules in the build: %d\n", len(all))
	verdict("ADR 12 as written", build(map[string]string{"docs/adr/0012-graph-is-a-projection.md": adr12NoRejected}, all))
	verdict("next PR with domain -> infra import", build(map[string]string{"domain/swap.go": domainBad}, all))
}
