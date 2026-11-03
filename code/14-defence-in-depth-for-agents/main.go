// Defence in depth for an agent, as pure decisions. This program only
// matches text. It never runs a command it reads. Five rings, four attempts,
// and each attempt stops at a different ring. With one ring, one wins.
package main

import (
	"fmt"
	"regexp"
	"strings"
)

type attempt struct {
	tool, command, target string
	irreversible          bool
	humanSaysYes          bool
}

type rings struct {
	allowedTools map[string]bool
	hook         *regexp.Regexp
	prodToken    string // "read" or "admin"
	sandboxRoot  string // what the sandbox mounts as HOME
	askHuman     bool
}

// check walks the rings in order. The first ring that refuses names itself.
func check(r rings, a attempt) string {
	if r.allowedTools != nil && !r.allowedTools[a.tool] {
		return "ring 1, tool allow-list: no " + a.tool + " tool"
	}
	if r.hook != nil && r.hook.MatchString(a.command) {
		return "ring 2, pre-tool hook: this deletes data volumes"
	}
	if strings.HasPrefix(a.target, "production") && r.prodToken == "read" {
		return "ring 3, credential: forbidden, the token can only read"
	}
	if strings.HasPrefix(a.target, "/home/operator") && r.sandboxRoot != "/home/operator" {
		return "ring 4, sandbox: " + a.target + " is not mounted"
	}
	if r.askHuman && a.irreversible && !a.humanSaysYes {
		return "ring 5, human: the operator said no"
	}
	return "REACHED " + a.target
}

func main() {
	five := rings{
		allowedTools: map[string]bool{"shell": true, "kubectl": true, "sql": true},
		hook:         regexp.MustCompile(`docker\s+compose\s+down\s+.*-v\b`),
		prodToken:    "read",
		sandboxRoot:  "/tmp/empty-home",
		askHuman:     true,
	}
	attempts := []attempt{
		{"shell", "docker compose down -v", "data volumes", true, false},
		{"kubectl", "kubectl delete pvc data", "production pvc", true, false},
		{"shell", "go test ./wipe-wallet", "/home/operator/.wallet", true, false},
		{"sql", "DROP TABLE old_staging", "staging table", true, false},
	}
	fmt.Println("Five rings")
	for _, a := range attempts {
		fmt.Printf("  %-26s -> %s\n", a.command, check(five, a))
	}
	fmt.Println("One ring: a system prompt, the operator's own keys")
	one := rings{prodToken: "admin", sandboxRoot: "/home/operator"}
	fmt.Printf("  %-26s -> %s\n", attempts[0].command, check(one, attempts[0]))
}
