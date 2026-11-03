// Cursor paging: nextCursor is opaque. Send it back unchanged. No nextCursor
// means the end. Run: go run .
package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

const pageSize = 50

var errBadCursor = errors.New("error -32602 invalid cursor")

// list answers tools/list. Inside, the cursor is base64 of a position.
func list(all []string, cursor string) (page []string, next string, err error) {
	start := 0
	if cursor != "" {
		raw, e := base64.StdEncoding.DecodeString(cursor)
		if e != nil || !strings.HasPrefix(string(raw), "pos=") {
			return nil, "", errBadCursor
		}
		fmt.Sscanf(string(raw), "pos=%d", &start)
	}
	end := min(start+pageSize, len(all))
	if end < len(all) {
		next = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("pos=%d", end)))
	}
	return all[start:end], next, nil
}

func main() {
	all := make([]string, 112)
	for i := range all {
		all[i] = fmt.Sprintf("tool_%03d", i)
	}
	cursor, n, got := "", 0, 0
	for {
		page, next, _ := list(all, cursor)
		n++
		got += len(page)
		fmt.Printf("%d tools/list cursor=%q -> %d tools, nextCursor=%q\n", n, cursor, len(page), next)
		if next == "" { // no nextCursor is the only end signal
			break
		}
		cursor = next // unchanged
	}
	fmt.Println("complete:", got, "tools")

	raw, _ := base64.StdEncoding.DecodeString("cG9zPTUw")
	fmt.Println("inside the cursor:", string(raw))
	_, _, err := list(all, "offset=100") // a host builds its own cursor
	fmt.Println("host-made cursor:", err)
}
