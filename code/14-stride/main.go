// STRIDE, letter by letter, over one boundary: our alert webhook to a
// customer's server. Each letter has one control, and the program proves it.
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/netip"
	"time"
)

var webhookKey = []byte("per-customer-key")

// S and T: sign each body, so a fake or a changed alert fails the check.
func sign(body []byte) string {
	m := hmac.New(sha256.New, webhookKey)
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}

func customerAccepts(body []byte, sig string) bool {
	return hmac.Equal([]byte(sign(body)), []byte(sig))
}

// R: one line per delivery means nobody can say it was never sent.
var deliveries []string

func logDelivery(customer string, status int, at string) {
	deliveries = append(deliveries, fmt.Sprintf("%s %d %s", customer, status, at))
}

// I: the payload holds only what the customer needs.
type alert struct {
	Address string `json:"address"`
	Risk    string `json:"risk"`
}

type alertBefore struct {
	alert
	History []string `json:"address_history"`
	CaseID  string   `json:"internal_case_id"`
}

// D: a slow customer costs one timeout, never everyone's queue.
func deliver(send func() int, timeout time.Duration) string {
	done := make(chan int, 1)
	go func() { done <- send() }()
	select {
	case s := <-done:
		return fmt.Sprint(s)
	case <-time.After(timeout):
		return "timed out, retry later, on this customer's own queue"
	}
}

// E: a callback may not point inside our network.
func callbackAllowed(ip string) bool {
	a := netip.MustParseAddr(ip)
	return !(a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast())
}

func main() {
	body, _ := json.Marshal(alert{"0xabc", "high"})
	sig := sign(body)
	fmt.Println("  S fake alert, no key     ->", customerAccepts([]byte(`{"risk":"high"}`), "bad"))
	fmt.Println("  S real alert             ->", customerAccepts(body, sig))
	fmt.Println("  T changed on the way     ->", customerAccepts([]byte(`{"address":"0xabc","risk":"low"}`), sig))
	logDelivery("customer-9", 200, "22:41")
	fmt.Println("  R log                    ->", deliveries)
	before, _ := json.Marshal(alertBefore{alert{"0xabc", "high"}, []string{"0x1", "0x2"}, "case-202"})
	fmt.Println("  I before                 ->", string(before))
	fmt.Println("  I after                  ->", string(body))
	fmt.Println("  D fast customer          ->", deliver(func() int { return 200 }, 200*time.Millisecond))
	fmt.Println("  D slow customer          ->", deliver(func() int { time.Sleep(time.Second); return 200 }, 200*time.Millisecond))
	fmt.Println("  E callback 10.0.0.5      ->", callbackAllowed("10.0.0.5"))
	fmt.Println("  E callback 93.184.215.14 ->", callbackAllowed("93.184.215.14"))
}
