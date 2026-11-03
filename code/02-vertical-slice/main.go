// Replays the vertical slice lesson: a local MCP proxy with three tools.
// Cut by layer, a new tool edits three folders and two developers collide.
// Cut by slice, it is one new folder. Then a rule copied into three slices
// drifts, and a shared package fixes it.
package main

import (
	"fmt"
	"path"
	"sort"
)

// foldersTouched counts the distinct folders a change edits.
func foldersTouched(files []string) []string {
	seen := map[string]bool{}
	for _, f := range files {
		seen[path.Dir(f)] = true
	}
	var out []string
	for d := range seen {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// conflicts lists the files two changes both edit.
func conflicts(a, b []string) []string {
	inA := map[string]bool{}
	for _, f := range a {
		inA[f] = true
	}
	var out []string
	for _, f := range b {
		if inA[f] {
			out = append(out, f)
		}
	}
	return out
}

// layerChange is a new tool cut by layer: one file per layer folder,
// plus the shared registry each layer folder keeps.
func layerChange(tool string) []string {
	var files []string
	for _, layer := range []string{"handler", "logic", "data"} {
		files = append(files, layer+"/"+tool+".go", layer+"/registry.go")
	}
	return files
}

// sliceChange is a new tool cut by slice: one new folder, nothing else.
func sliceChange(tool string) []string {
	return []string{
		tool + "/handler.go", tool + "/logic.go",
		tool + "/query.go", tool + "/tool_test.go",
	}
}

// A slice's own copy of the network check.
type checkNetwork func(network string) bool

func copyOf(nets ...string) checkNetwork {
	ok := map[string]bool{}
	for _, n := range nets {
		ok[n] = true
	}
	return func(n string) bool { return ok[n] }
}

func main() {
	fmt.Println("cut by layer: add price_lookup")
	a := layerChange("price_lookup")
	fmt.Printf("  files edited: %d, folders touched: %v (%d)\n", len(a), foldersTouched(a), len(foldersTouched(a)))
	b := layerChange("token_info") // a second developer, same day
	fmt.Printf("  second developer adds token_info: merge conflicts in %v\n", conflicts(a, b))

	fmt.Println("cut by slice: add price_lookup")
	a = sliceChange("price_lookup")
	b = sliceChange("token_info")
	fmt.Printf("  files edited: %d, folders touched: %v (%d)\n", len(a), foldersTouched(a), len(foldersTouched(a)))
	fmt.Printf("  second developer adds token_info: merge conflicts %d\n", len(conflicts(a, b)))

	// The network check was copied into three slices. base was added to one copy.
	slices := map[string]checkNetwork{
		"graph_query":        copyOf("robinhood", "base"),
		"wallet_balance":     copyOf("robinhood"),
		"subscription_topup": copyOf("robinhood"),
	}
	names := []string{"graph_query", "wallet_balance", "subscription_topup"}
	refuse := func() (out []string) {
		for _, n := range names {
			if !slices[n]("base") {
				out = append(out, n)
			}
		}
		return
	}
	fmt.Printf("copied check, 3 copies, 1 updated: tools refusing network base: %v\n", refuse())

	shared := copyOf("robinhood", "base") // shared/network.go, one copy
	for _, n := range names {
		slices[n] = shared // every slice calls the shared check
	}
	fmt.Printf("shared check, 1 copy: tools refusing network base: %d\n", len(refuse()))
}
