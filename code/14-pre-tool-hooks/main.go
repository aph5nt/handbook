// A pre-tool hook as a pure decision. This program matches text only: it
// never runs any command it reads. The exit codes follow the Claude Code
// rule: 0 allows the call and 2 blocks it, with the reason shown to the model.
package main

import (
	"encoding/json"
	"fmt"
	"regexp"
)

type toolCall struct {
	Tool  string `json:"tool_name"`
	Input struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

type rule struct {
	pattern *regexp.Regexp
	reason  string
}

// denyList holds the commands that destroy protected data.
var denyList = []rule{
	{regexp.MustCompile(`docker\s+compose\s+down\s+.*-v\b`), "this deletes protected data volumes"},
	{regexp.MustCompile(`docker\s+volume\s+rm\b`), "this deletes protected data volumes"},
}

// hook reads the tool call as JSON and answers an exit code and a reason.
func hook(raw string) (int, string) {
	var c toolCall
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return 2, "cannot read the tool call, so refuse it"
	}
	if c.Tool != "Bash" {
		return 0, ""
	}
	for _, r := range denyList {
		if r.pattern.MatchString(c.Input.Command) {
			return 2, "blocked: " + r.reason
		}
	}
	return 0, ""
}

func call(command string) string {
	b, _ := json.Marshal(map[string]any{"tool_name": "Bash", "tool_input": map[string]string{"command": command}})
	return string(b)
}

// sandboxAllows is the next ring: only the work folder is mounted, so a
// protected path is not there to reach.
func sandboxAllows(path string) bool {
	return regexp.MustCompile(`^/work/`).MatchString(path)
}

func show(command string) {
	code, why := hook(call(command))
	fmt.Printf("  %-28s -> exit %d %s\n", command, code, why)
}

func main() {
	show("docker compose down -v")
	show("docker system df")
	show("bash cleanup.sh")
	fmt.Println("  the pattern saw only the file name in that last call")
	fmt.Println("  sandbox: /work/report.txt reachable?", sandboxAllows("/work/report.txt"))
	fmt.Println("  sandbox: /var/lib/docker/volumes reachable?", sandboxAllows("/var/lib/docker/volumes"))
}
