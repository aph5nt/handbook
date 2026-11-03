// A resource template is one entry with variables. completion/complete
// suggests values for a variable or a prompt argument. Run: go run .
package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var networks = []string{"base", "ethereum", "robinhood"}

var schemas = map[string]string{"robinhood": "labels: Address, FLOWS_TO", "base": "labels: Address"}

const template = "graph://{network}/schema"

// complete answers completion/complete for the argument "network".
func complete(partial string) (values []string, total int) {
	for _, n := range networks {
		if strings.HasPrefix(n, partial) {
			values = append(values, n)
		}
	}
	sort.Strings(values)
	return values, len(values)
}

// expand fills each {name} in the template (RFC 6570 level 1).
func expand(tpl string, vars map[string]string) string {
	return regexp.MustCompile(`\{(\w+)\}`).ReplaceAllStringFunc(tpl, func(m string) string {
		return vars[m[1:len(m)-1]]
	})
}

// read answers resources/read for a URI that matches the template.
func read(uri string) string {
	m := regexp.MustCompile(`^graph://(\w+)/schema$`).FindStringSubmatch(uri)
	if m == nil {
		return "error -32002 resource not found"
	}
	return schemas[m[1]]
}

func main() {
	fmt.Println("0 resources/templates/list:", template)
	v, n := complete("rob")
	fmt.Printf("2 completion/complete network=rob -> values %v total %d\n", v, n)
	uri := expand(template, map[string]string{"network": v[0]})
	fmt.Println("3 resources/read", uri, "->", read(uri))
	v, n = complete("ba")
	fmt.Printf("4 prompt /trace-funds net=ba -> values %v total %d\n", v, n)

	var perRow []string // (bug) one resource per address
	for i := 0; i < 40000; i++ {
		perRow = append(perRow, fmt.Sprintf("graph://robinhood/address/%d", i))
	}
	fmt.Println("5 one resource per row:", len(perRow), "entries in resources/list; with a template: 1")
}
