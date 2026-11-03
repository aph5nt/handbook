// Futures and pipe-to: a slow call must come back as a message, never as a callback
// that writes the actor's state from another goroutine.
package main

import (
	"fmt"
	"time"
)

type Msg interface{}

type Config struct{ IntervalS, LimitC int }
type Reading struct {
	C    float64
	From string // the sender of this message
}
type ConfigLoaded struct {
	Cfg       Config
	Requester string // carried in the message, not read from "Sender"
}

var start = time.Now()

func log(f string, a ...any) {
	fmt.Printf("%4dms "+f+"\n", append([]any{time.Since(start).Milliseconds()}, a...)...)
}

// loadConfig is the async database call. It returns a future: a channel that gets one value.
func loadConfig(d time.Duration) <-chan Config {
	fut := make(chan Config, 1)
	go func() { time.Sleep(d); fut <- Config{IntervalS: 30, LimitC: 25} }()
	return fut
}

// pipeTo turns a completed future into a message in the mailbox. It touches nothing else.
func pipeTo(fut <-chan Config, mailbox chan Msg, requester string) {
	go func() { mailbox <- ConfigLoaded{<-fut, requester} }()
}

func main() {
	fmt.Println("== pipe-to")
	mailbox := make(chan Msg, 8)
	cfg := Config{} // device 42's state
	done := make(chan struct{})
	go func() {
		for m := range mailbox {
			switch v := m.(type) {
			case string: // "start"
				fut := loadConfig(200 * time.Millisecond)
				pipeTo(fut, mailbox, "api")
				log("device 42: LoadConfig started, pipe-to attached, handler ends")
			case Reading:
				log("device 42: handled reading %.1f C while the database works", v.C)
			case ConfigLoaded:
				cfg = v.Cfg // state changes inside a handler
				log("device 42: ConfigLoaded -> interval %d s, reply to %s", cfg.IntervalS, v.Requester)
				close(done)
			}
		}
	}()
	mailbox <- "start"
	time.Sleep(50 * time.Millisecond)
	mailbox <- Reading{21.5, "sensor"}
	<-done

	fmt.Println("\n== callback that writes the state")
	shared := Config{IntervalS: 60, LimitC: 30} // the old config
	var sender string                           // valid only during a handler
	halfway := make(chan struct{})
	written := make(chan struct{})
	callbackDone := make(chan struct{})
	// The handler for Reading 21.9 C, on "thread 1".
	go func() {
		sender = "sensor"
		interval := shared.IntervalS // reads the old interval, 60
		halfway <- struct{}{}
		<-written
		limit := shared.LimitC // reads the new limit, 25
		log("thread 1: alarm check used interval %d s and limit %d C: a config that never existed", interval, limit)
		<-callbackDone
	}()
	// The callback, on "thread 2", when the database answers.
	<-halfway
	shared = Config{IntervalS: 30, LimitC: 25}
	log("thread 2: callback wrote config while the handler ran")
	written <- struct{}{}
	log("thread 2: callback replies to Sender = %q, but the config was asked for by %q", sender, "api")
	close(callbackDone)
	time.Sleep(10 * time.Millisecond)

	fmt.Println("\n== the fix: pipe it back with the requester inside")
	mailbox2 := make(chan Msg, 8)
	done2 := make(chan struct{})
	go func() {
		for m := range mailbox2 {
			if v, ok := m.(ConfigLoaded); ok {
				log("device 42: applied interval %d s, limit %d C in one handler, reply to %s", v.Cfg.IntervalS, v.Cfg.LimitC, v.Requester)
				close(done2)
			}
		}
	}()
	pipeTo(loadConfig(100*time.Millisecond), mailbox2, "api")
	<-done2
}
