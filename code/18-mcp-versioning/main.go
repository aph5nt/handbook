// Version negotiation: the client proposes its newest, the server answers with
// one it supports, and the client adapts or leaves. Run: go run .
package main

import (
	"fmt"
	"slices"
)

// Features by the version that added them.
var addedIn = map[string]string{"elicitation/create": "2025-06-18", "tools/call": "2024-11-05"}

// negotiate is the server side: echo the version if known, else its newest.
func negotiate(proposed string, supported []string) string {
	if slices.Contains(supported, proposed) {
		return proposed
	}
	return supported[len(supported)-1]
}

// available: dates in this form compare as strings.
func available(method, agreed string) bool { return addedIn[method] <= agreed }

type Client struct {
	supported []string
	agreed    string
	obeys     bool
}

// handshake returns false if the client must disconnect.
func (c *Client) handshake(serverVersions []string) bool {
	proposed := c.supported[len(c.supported)-1]
	got := negotiate(proposed, serverVersions)
	fmt.Println("  proposes", proposed, "-> server answers", got)
	if !slices.Contains(c.supported, got) {
		return false
	}
	c.agreed = got
	return true
}

func (c *Client) call(method string, server []string) string {
	if c.obeys && !available(method, c.agreed) {
		return method + " turned off for this server"
	}
	if addedIn[method] > server[len(server)-1] {
		return "error -32601 unknown method: " + method
	}
	return "POST /mcp, MCP-Protocol-Version: " + c.agreed + ", method " + method
}

func main() {
	older := []string{"2024-11-05", "2025-03-26"}
	c := &Client{supported: []string{"2025-03-26", "2025-06-18"}, obeys: true}
	fmt.Println("handshake ok:", c.handshake(older))
	fmt.Println(c.call("elicitation/create", older))
	fmt.Println(c.call("tools/call", older))
	c.obeys = false
	fmt.Println("buggy client:", c.call("elicitation/create", older))

	tooOld := &Client{supported: []string{"2025-06-18"}}
	fmt.Println("handshake ok:", tooOld.handshake([]string{"2024-11-05"}), "-> disconnect")
}
