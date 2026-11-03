// Correlation id: one id across a whole request, carried by middleware.
//
// A Go map stands in for message headers, and a slice for the shared log
// store that support searches. Each service is a function. Deterministic.
package main

import (
	"fmt"
	"strings"
)

type Msg struct {
	Name    string
	Headers map[string]string
}

var logs []string

func log(service, cid, text string) {
	logs = append(logs, fmt.Sprintf("%s [%s] %s", service, cid, text))
}

var nextID = 76

// edge sets the id once, only if the request has none.
func edge(h map[string]string) string {
	if c := h["correlation_id"]; c != "" {
		return c
	}
	nextID++
	return fmt.Sprintf("c-%d", nextID)
}

// carry is the shared middleware: it copies the id to the next message.
func carry(in Msg, name string) Msg {
	return Msg{name, map[string]string{
		"correlation_id": in.Headers["correlation_id"],
	}}
}

func search(cid string) (n int) {
	for _, l := range logs {
		if strings.Contains(l, "["+cid+"]") {
			n++
		}
	}
	return
}

// flow runs the four services. brokenUnits makes the units service mint its own id.
func flow(brokenUnits bool) {
	logs = nil
	nextID = 76
	cid := edge(map[string]string{})
	log("gateway", cid, "POST /topup")
	paid := Msg{"TopUpPaid", map[string]string{"correlation_id": cid}}
	log("top-up", cid, "charged, publish TopUpPaid")

	units := carry(paid, "UnitsGranted")
	if brokenUnits {
		units.Headers["correlation_id"] = "u-9" // written by hand, a new id
	}
	log("units", units.Headers["correlation_id"], "granted")

	receipt := carry(units, "ReceiptSent")
	log("receipt", receipt.Headers["correlation_id"], "email sent")
}

func main() {
	fmt.Println("== Tab 1: carried ==")
	flow(false)
	for _, l := range logs {
		fmt.Println(l)
	}
	n := search("c-77")
	fmt.Printf("support searches c-77: %d of 4 log lines\n", n)

	fmt.Println("== Tab 2: broken ==")
	flow(true)
	for _, l := range logs {
		fmt.Println(l)
	}
	fmt.Printf("support searches c-77: %d of 4 log lines, the trail stops\n", search("c-77"))
}
