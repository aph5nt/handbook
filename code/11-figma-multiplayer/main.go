// Command figma replays the Figma lesson: last write wins per property, a
// parent loop refused by walking up the tree, and a proxy that routes by key
// and fans out when there is none.
package main

import (
	"fmt"
	"hash/fnv"
)

type File struct {
	Props  map[string]string // "object.property" -> value
	Parent map[string]string
}

func (f *File) Apply(object, prop, value string) {
	f.Props[object+"."+prop] = value
}

// Browser shows an edit at once, then takes what the server says.
func Browser(mine, server string) (first, final string) {
	return mine, server
}

func (f *File) Move(node, newParent string) error {
	for p := newParent; p != ""; p = f.Parent[p] {
		if p == node {
			return fmt.Errorf("move %s into %s makes a loop: undo", node, newParent)
		}
	}
	f.Parent[node] = newParent
	return nil
}

// HomeOf is the first step: rarely joined tables get their own database.
func HomeOf(table string) string {
	switch table {
	case "users", "teams", "billing":
		return "db-2"
	}
	return "db-1"
}

func DBProxy(fileID string, shards int) []int {
	if fileID != "" {
		h := fnv.New32a()
		h.Write([]byte(fileID))
		return []int{int(h.Sum32())%shards + 1}
	}
	all := []int{}
	for i := 1; i <= shards; i++ {
		all = append(all, i)
	}
	return all
}

func main() {
	f := &File{Props: map[string]string{}, Parent: map[string]string{"Group A": "Page", "Group B": "Page"}}
	f.Apply("button", "colour", "blue")
	f.Apply("button", "text", "Book now")
	fmt.Println("mp step 2:", f.Props["button.colour"], "and", f.Props["button.text"])
	f.Apply("button", "colour", "red")
	f.Apply("button", "colour", "green")
	fmt.Println("mp step 3: Ana red, then Ben green:", f.Props["button.colour"])
	a, b := Browser("red", f.Props["button.colour"])
	fmt.Println("mp step 4: Ana's screen:", a, "then", b)
	fmt.Println("mp step 5-6: Ana's move:", f.Move("Group A", "Group B"))
	fmt.Println("mp step 6: Ben's move:", f.Move("Group B", "Group A"))
	fmt.Println("mp step 6: Group A in", f.Parent["Group A"], "; Group B in", f.Parent["Group B"])
	fmt.Println("pg step 5: file f-42 asks shards:", DBProxy("f-42", 2))
	fmt.Println("pg step 6: all files a user can see asks shards:", DBProxy("", 2))
}
