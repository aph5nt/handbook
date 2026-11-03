// Logging over the protocol: the client sets a level, the server drops what is
// below it, and a secret never goes into a log line. Run: go run .
package main

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// RFC 5424 order, lowest to highest.
var levels = map[string]int{"debug": 0, "info": 1, "notice": 2, "warning": 3, "error": 4, "critical": 5, "alert": 6, "emergency": 7}

type Logger struct {
	min    string
	redact bool
	send   func(string)
}

func (l *Logger) setLevel(level string) { l.min = level } // logging/setLevel

var bearer = regexp.MustCompile(`Bearer [\w.-]+`)

func (l *Logger) log(level, data string) {
	if levels[level] < levels[l.min] {
		fmt.Printf("  (dropped, %s is below %s) %s\n", level, l.min, data)
		return
	}
	if l.redact {
		data = bearer.ReplaceAllString(data, "Bearer [redacted]")
	}
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "notifications/message",
		"params": map[string]string{"level": level, "logger": "proxy", "data": data}})
	l.send(string(b))
}

func main() {
	fmt.Println(`0 initialize: capabilities {"logging":{}}`)
	l := &Logger{min: "info", send: func(s string) { fmt.Println("  <-", s) }}
	l.setLevel("warning")
	fmt.Println("1 logging/setLevel warning")
	l.log("warning", "lane is 40 blocks behind")
	l.log("debug", "cache hit")
	l.setLevel("debug")
	fmt.Println("4 logging/setLevel debug")
	l.log("debug", "cache hit")
	req := "request with Bearer eyJhbGciOi.abc"
	fmt.Println("5 careless server:")
	l.log("debug", req)
	l.redact = true
	fmt.Println("5 fixed server:")
	l.log("debug", req)
}
