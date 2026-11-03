// Module: group code by business idea, not by technical layer.
// Count the packages one change touches, and refuse calls into private parts.
package main

import (
	"fmt"
	"sort"
	"strings"
)

// File is one source file: its package, the idea it serves, and what it exports.
type File struct {
	Pkg, Idea, Name string
	Public          bool
}

// Touched lists the packages a change to one business idea must edit.
func Touched(files []File, idea string) []string {
	set := map[string]bool{}
	for _, f := range files {
		if f.Idea == idea {
			set[f.Pkg] = true
		}
	}
	var out []string
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// Call checks one cross-module call: only the public surface may be used.
func Call(files []File, from, pkg, name string) error {
	for _, f := range files {
		if f.Pkg == pkg && f.Name == name && (f.Public || from == pkg) {
			return nil
		}
	}
	return fmt.Errorf("%s may not call %s.%s: private", from, pkg, name)
}

func byLayer() []File {
	var fs []File
	for _, idea := range []string{"swap", "price", "transfer"} {
		for _, layer := range []string{"entities", "services", "repositories", "handlers"} {
			fs = append(fs, File{Pkg: layer, Idea: idea, Name: idea + "_" + layer, Public: true})
		}
	}
	return fs
}

func byIdea() []File {
	return []File{
		{Pkg: "dex", Idea: "swap", Name: "Swaps", Public: true},
		{Pkg: "dex", Idea: "swap", Name: "decoder"},
		{Pkg: "dex", Idea: "swap", Name: "tables"},
		{Pkg: "pricing", Idea: "price", Name: "DayPrice", Public: true},
		{Pkg: "transfers", Idea: "transfer", Name: "Transfers", Public: true},
	}
}

func main() {
	fmt.Println("by layer, swap fee change touches:", strings.Join(Touched(byLayer(), "swap"), ", "))
	fmt.Println("by idea, swap fee change touches:", strings.Join(Touched(byIdea(), "swap"), ", "))
	fmt.Println("pricing -> dex.Swaps:", Call(byIdea(), "pricing", "dex", "Swaps"))
	fmt.Println("pricing -> dex.decoder:", Call(byIdea(), "pricing", "dex", "decoder"))
	fmt.Println("modules in the indexer context:", Touched(byIdea(), "swap"), Touched(byIdea(), "price"), Touched(byIdea(), "transfer"))
}
