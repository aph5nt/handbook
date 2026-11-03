// Context map: boxes are contexts, arrows run from upstream to downstream, and
// the name on each arrow says who must warn whom when something changes.
package main

import "fmt"

// Arrow is one border on the map. Rel "" means nobody named it.
type Arrow struct {
	Up, Down, Rel string
}

// Map is the whole picture: contexts and named arrows.
var Map = []Arrow{
	{"indexer", "graph sync", "shared kernel"},
	{"warehouse", "graph sync", "customer-supplier"},
	{"Ethereum node", "indexer", "conformist"},
	{"price vendor", "warehouse", "anti-corruption layer"},
	{"graph sync", "agent hosts", "open host service"},
	{"indexer", "analysts", "published language"},
	{"billing", "graph", "separate ways"},
}

// Unnamed finds the arrows nobody labelled: the map's real defects.
func Unnamed(m []Arrow) []Arrow {
	var bad []Arrow
	for _, a := range m {
		if a.Rel == "" {
			bad = append(bad, a)
		}
	}
	return bad
}

// RenameEffect says what an upstream column rename does, by the arrow's name.
func RenameEffect(a Arrow) string {
	switch a.Rel {
	case "customer-supplier":
		return "planned: add value, move the reader, drop amount"
	case "anti-corruption layer":
		return "one adapter changes"
	case "shared kernel":
		return "both teams must approve"
	case "separate ways":
		return "nothing happens: no link"
	case "conformist":
		return "every reader changes, with no warning"
	case "":
		return "breaks at night: nobody knew who had to warn whom"
	}
	return "readers keep the documented door"
}

func main() {
	fmt.Println("arrows:", len(Map), "unnamed:", len(Unnamed(Map)))
	for _, a := range Map {
		fmt.Printf("%-14s -> %-12s %s\n", a.Up, a.Down, a.Rel)
	}
	noName := Arrow{"warehouse", "graph sync", ""}
	fmt.Println("no name:", RenameEffect(noName), "- unnamed:", len(Unnamed([]Arrow{noName})))
	named := Arrow{"warehouse", "graph sync", "customer-supplier"}
	fmt.Println("named:", RenameEffect(named))
	fmt.Println("vendor renames a field:", RenameEffect(Map[3]))
}
