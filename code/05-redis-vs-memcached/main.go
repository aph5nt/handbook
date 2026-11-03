// Redis versus Memcached, as a model in work units (1 unit = one key touched).
// Memcached: many threads, plain bytes. Redis: one main thread runs commands
// one after another, so each is atomic, and one slow command stalls everyone.
package main

import (
	"fmt"
	"sort"
)

// ---- Memcached: one worker per core, opaque values ----

type Memcached struct {
	threads int
	data    map[string][]byte // values are blobs it never looks inside
}

// Serve returns the time, in units, to serve n gets of cost 1.
func (m *Memcached) Serve(n int) int {
	return (n + m.threads - 1) / m.threads
}

func (m *Memcached) Restart() {
	m.data = map[string][]byte{} // a pure cache: empty
}

// ---- Redis: one main thread, a queue, data structures ----

type cmd struct {
	name string
	cost int // units of work on the main thread
}

type Redis struct {
	counters map[string]int
	expires  map[string]int
	zset     map[string]float64
	subs     []chan string
}

// Run executes the queue in order on one thread. It returns each command's
// finish time: a command waits for everything queued before it.
func Run(queue []cmd) map[string]int {
	t, done := 0, map[string]int{}
	for _, c := range queue {
		t += c.cost // nothing else runs while this command runs
		done[c.name] = t
	}
	return done
}

func (r *Redis) Incr(k string) int        { r.counters[k]++; return r.counters[k] }
func (r *Redis) Expire(k string, sec int) { r.expires[k] = sec }
func (r *Redis) ZIncrBy(m string, d float64) {
	r.zset[m] += d
}
func (r *Redis) Publish(msg string) {
	for _, s := range r.subs {
		s <- msg // every listening instance gets it
	}
}

func main() {
	mc := &Memcached{threads: 4, data: map[string][]byte{"risk:0x77": []byte("high")}}
	units := mc.Serve(8)
	fmt.Printf("Memcached, 4 threads: 8 gets take %d units\n", units)
	mc.Restart()
	fmt.Printf("Memcached after a restart: %d keys\n", len(mc.data))
	var gets []cmd
	for i := 0; i < 8; i++ {
		gets = append(gets, cmd{fmt.Sprint("GET ", i), 1})
	}
	fmt.Printf("Redis, 1 main thread: 8 gets take %d units\n\n", Run(gets)["GET 7"])

	r := &Redis{counters: map[string]int{}, expires: map[string]int{}, zset: map[string]float64{}}
	limit, refused := 100, 0
	for i := 0; i < 103; i++ { // wallet 0xd8 sends 103 requests in a minute
		if r.Incr("rl:0xd8") == 1 {
			r.Expire("rl:0xd8", 60)
		}
		if r.counters["rl:0xd8"] > limit {
			refused++
		}
	}
	fmt.Printf("rate limit 100 a minute: counter=%d, TTL=%d s, refused=%d\n", r.counters["rl:0xd8"], r.expires["rl:0xd8"], refused)
	for _, a := range []string{"0x77", "0x18", "0x77", "0x5e", "0x77", "0x18"} {
		r.ZIncrBy(a, 1)
	}
	top := []string{"0x77", "0x18", "0x5e"}
	sort.Slice(top, func(i, j int) bool { return r.zset[top[i]] > r.zset[top[j]] })
	fmt.Printf("most screened (sorted set): %v scores %v %v %v\n", top, r.zset[top[0]], r.zset[top[1]], r.zset[top[2]])
	a, b := make(chan string, 1), make(chan string, 1)
	r.subs = []chan string{a, b}
	r.Publish("drop risk:0x77")
	fmt.Printf("PUBLISH reached instance A: %q and B: %q\n\n", <-a, <-b)

	const keys = 1_000_000
	slow := Run([]cmd{{"KEYS *", keys}, {"GET risk:0x77", 1}})
	fmt.Printf("KEYS * over %d keys, then a GET: the GET finishes at unit %d\n", keys, slow["GET risk:0x77"])
	var q []cmd
	for i := 0; i < keys/100; i++ {
		q = append(q, cmd{fmt.Sprint("SCAN step ", i), 100})
		if i == 0 {
			q = append(q, cmd{"GET risk:0x77", 1}) // the GET slips in after one step
		}
	}
	fmt.Printf("SCAN 100 at a time: the GET finishes at unit %d\n", Run(q)["GET risk:0x77"])
	del := Run([]cmd{{"DEL bigset", 5_000_000}, {"GET risk:0x77", 1}})
	unlink := Run([]cmd{{"UNLINK bigset", 1}, {"GET risk:0x77", 1}}) // memory freed on a background thread
	fmt.Printf("delete a 5M-member set: DEL makes the GET wait until unit %d, UNLINK until unit %d\n",
		del["GET risk:0x77"], unlink["GET risk:0x77"])
}
