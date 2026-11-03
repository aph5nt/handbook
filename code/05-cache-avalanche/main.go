// Cache avalanche: 20 keys set at 09:00 with the same 1-hour TTL all expire
// at 10:00. Jitter spreads them. Then the whole cache dies, and a limiter
// in front of the database lets through only what it can serve.
package main

import (
	"errors"
	"fmt"
	"math/rand"
	"time"
)

var nineAM = time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)

// expiries returns when each of the 20 keys expires.
func expiries(jitter bool, r *rand.Rand) []time.Time {
	out := make([]time.Time, 20)
	for i := range out {
		ttl := time.Hour
		if jitter {
			ttl += time.Duration(r.Int63n(int64(6 * time.Minute))) // + 0 to 6 minutes
		}
		out[i] = nineAM.Add(ttl)
	}
	return out
}

func perMinute(exp []time.Time) map[string]int {
	m := map[string]int{}
	for _, t := range exp {
		m[t.Format("15:04")]++
	}
	return m
}

// Limiter lets at most perSecond requests reach the database each second.
type Limiter struct {
	perSecond, used int
}

func (l *Limiter) Allow() bool {
	if l.used < l.perSecond {
		l.used++
		return true
	}
	return false // served a stale local copy or a quick error instead
}

// A small store for the read path: the cache, a stale local copy, a database.
type store map[string]string

func (s store) Get(k string) (string, bool)        { v, ok := s[k]; return v, ok }
func (s store) Set(k, v string, ttl time.Duration) { s[k] = v }

type database struct{ loads int }

func (d *database) Load(k string) (string, error) { d.loads++; return "row " + k, nil }

var (
	cache      = store{}
	localStale = store{}
	db         = &database{}
	limiter    = &Limiter{perSecond: 300}
	jitter     = true
	errBusy    = errors.New("busy, try later")
)

func load(k string) string { v, _ := db.Load(k); return v }

func warm(keys []string, r *rand.Rand) {
	for _, k := range keys {
		ttl := time.Hour
		if jitter {
			ttl += time.Duration(r.Int63n(int64(6 * time.Minute))) // + 0 to 6 minutes
		}
		cache.Set(k, load(k), ttl)
	}
}

func read(k string) (string, error) {
	if v, ok := cache.Get(k); ok {
		return v, nil // hit
	}
	if limiter.Allow() { // at most 300 a second reach the database
		return db.Load(k)
	}
	if v, ok := localStale.Get(k); ok {
		return v, nil // a stale copy beats a dead database
	}
	return "", errBusy // a quick error, not a long wait
}

func main() {
	r := rand.New(rand.NewSource(42))
	fmt.Println("== same TTL for every key ==")
	fmt.Println("  keys expiring per minute:", perMinute(expiries(false, r)))
	fmt.Println("== TTL of 1 hour plus 0 to 6 minutes of jitter ==")
	for _, m := range []string{"10:00", "10:01", "10:02", "10:03", "10:04", "10:05"} {
		fmt.Printf("  %s: %d keys expire\n", m, perMinute(expiries(true, rand.New(rand.NewSource(42))))[m])
	}

	fmt.Println("== Redis dies: 2,000 reads a second, the database serves 300 ==")
	const reads, dbCapacity = 2000, 300
	fmt.Printf("  no protection: %d reads/s hit the database (%.1fx its capacity)\n", reads, float64(reads)/dbCapacity)
	warm([]string{"fee", "rate"}, r)
	for k := range cache {
		localStale[k] = cache[k] // the local copy, from before the crash
		delete(cache, k)         // Redis is gone
	}
	db.loads = 0
	copies, busy := 0, 0
	for i := 0; i < reads; i++ {
		k := []string{"fee", "other"}[i%2] // half have a local copy
		before := db.loads
		_, err := read(k)
		switch {
		case err != nil:
			busy++
		case db.loads == before:
			copies++
		}
	}
	fmt.Printf("  with a limiter: %d reach the database, %d get a stale copy or a quick error (%d copies, %d errors)\n", db.loads, copies+busy, copies, busy)
	fmt.Println("  with a replica: promoted in seconds, keys still there, the database stays at its normal share")
}
