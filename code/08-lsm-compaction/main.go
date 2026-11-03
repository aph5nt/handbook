// A small real LSM store, in memory: a memtable, sorted immutable files at
// level 0, and compaction into one level 1 file. It needs no database. It counts
// the places a read must check.
package main

import (
	"fmt"
	"sort"
)

type Entry struct {
	Key, Value string
	Deleted    bool
}

type LSM struct {
	memCap  int
	mem     map[string]Entry
	level0  [][]Entry // newest last
	level1  []Entry
	flushes int
}

func New(c int) *LSM { return &LSM{memCap: c, mem: map[string]Entry{}} }

func (l *LSM) Put(k, v string) { l.write(Entry{Key: k, Value: v}) }
func (l *LSM) Del(k string)    { l.write(Entry{Key: k, Deleted: true}) }

func (l *LSM) write(e Entry) {
	l.mem[e.Key] = e // the log write is not modelled, it adds no read cost
	if len(l.mem) >= l.memCap {
		l.flush()
	}
}

func (l *LSM) flush() {
	f := make([]Entry, 0, len(l.mem))
	for _, e := range l.mem {
		f = append(f, e)
	}
	sort.Slice(f, func(i, j int) bool { return f[i].Key < f[j].Key })
	l.level0 = append(l.level0, f)
	l.mem = map[string]Entry{}
	l.flushes++
}

// Get checks the memtable, then level 0 newest first, then level 1.
func (l *LSM) Get(k string) (string, int) {
	checks := 1
	if e, ok := l.mem[k]; ok {
		return e.Value, checks
	}
	for i := len(l.level0) - 1; i >= 0; i-- {
		checks++
		f := l.level0[i]
		p := sort.Search(len(f), func(j int) bool { return f[j].Key >= k })
		if p < len(f) && f[p].Key == k {
			return f[p].Value, checks
		}
	}
	if len(l.level1) == 0 {
		return "", checks
	}
	checks++
	p := sort.Search(len(l.level1), func(j int) bool { return l.level1[j].Key >= k })
	if p < len(l.level1) && l.level1[p].Key == k {
		return l.level1[p].Value, checks
	}
	return "", checks
}

// Compact merges level 0 into level 1. The newest version wins, deletes vanish.
func (l *LSM) Compact() (before, after int) {
	merged := map[string]Entry{}
	for _, e := range l.level1 {
		merged[e.Key] = e
	}
	for _, f := range l.level0 { // oldest first, so newer files overwrite
		for _, e := range f {
			before++
			merged[e.Key] = e
		}
	}
	out := make([]Entry, 0, len(merged))
	for _, e := range merged {
		if !e.Deleted {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	l.level1, l.level0 = out, nil
	return before, len(out)
}

func main() {
	l := New(4)
	for f := 0; f < 4; f++ {
		for k := 0; k < 4; k++ {
			if f == 3 && k == 3 {
				l.Del("key-1") // a delete is a write too: a marker, not a removal
				continue
			}
			l.Put(fmt.Sprintf("key-%d", k+f*2), fmt.Sprintf("v%d", f))
		}
	}
	v, c := l.Get("missing")
	fmt.Printf("after %d flushes: a read of a missing key checks %d places, level0 files %d (value %q)\n", l.flushes, c, len(l.level0), v)
	before, after := l.Compact()
	fmt.Printf("compaction: 4 files, %d entries in, %d out\n", before, after)
	_, c = l.Get("missing")
	fmt.Printf("after compaction: the same read checks %d places\n", c)
	v, _ = l.Get("key-6")
	fmt.Printf("key-6 keeps the newest version: %s\n", v)
	v, _ = l.Get("key-1")
	fmt.Printf("key-1 was deleted, and compaction dropped it: value %q\n", v)

	const versionLimit = 1000
	fmt.Printf("tablet limit %d versions, compaction removes 300 files per round\n", versionLimit)
	for _, loads := range []int{400, 1} {
		files, rounds := 0, 0
		for rounds < 100 {
			rounds++
			files += loads
			if files >= versionLimit {
				break
			}
			files -= min(files, 300)
		}
		if files >= versionLimit {
			fmt.Printf("  %4d files per round: refused after round %d: too many versions\n", loads, rounds)
		} else {
			fmt.Printf("  %4d files per round: %d files left after %d rounds\n", loads, files, rounds)
		}
	}
}
