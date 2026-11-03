// Vector store as a projection: vectors are built from facts by a model,
// stamped with the model version. A new model means a new index built
// beside the old one. Mixing models in one index returns nonsense quietly.
package main

import (
	"fmt"
	"math"
	"sort"
)

type Vector struct {
	Addr  string
	Model string
	X, Y  float64 // a real embedding has hundreds of numbers, two keep it visible
}

// Index holds vectors of one model. guard refuses a vector of another model.
type Index struct {
	Model string
	Vecs  []Vector
	guard bool
}

func (ix *Index) Add(v Vector) error {
	if ix.guard && v.Model != ix.Model {
		return fmt.Errorf("refused %s: model %s in a %s index", v.Addr, v.Model, ix.Model)
	}
	ix.Vecs = append(ix.Vecs, v)
	return nil
}

// Nearest returns the k closest vectors to q, by distance.
func (ix *Index) Nearest(q Vector, k int) []Vector {
	out := []Vector{}
	for _, v := range ix.Vecs {
		if v.Addr != q.Addr {
			out = append(out, v)
		}
	}
	d := func(v Vector) float64 { return math.Hypot(v.X-q.X, v.Y-q.Y) }
	sort.Slice(out, func(i, j int) bool { return d(out[i]) < d(out[j]) })
	if len(out) > k {
		out = out[:k]
	}
	return out
}

// Build embeds every address from the facts with one model.
func Build(model string, facts map[string][2]float64) *Index {
	ix := &Index{Model: model, guard: true}
	for _, a := range sortedKeys(facts) {
		p := facts[a]
		ix.Add(Vector{a, model, p[0], p[1]})
	}
	return ix
}

func sortedKeys(m map[string][2]float64) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func names(vs []Vector) string {
	s := ""
	for _, v := range vs {
		s += fmt.Sprintf(" %s(%s)", v.Addr, v.Model)
	}
	return s
}

func main() {
	// v1 embeddings of seven addresses, from the facts
	factsV1 := map[string][2]float64{"a0": {520, 90}, "scam": {540, 120}, "a2": {560, 100},
		"a3": {600, 200}, "a4": {620, 230}, "a5": {590, 250}, "a6": {530, 240}}
	v1 := Build("v1", factsV1)
	q := Vector{"scam", "v1", 540, 120}
	fmt.Println("v1 index, nearest 2 to the scam address:", names(v1.Nearest(q, 2)))

	// the trap: new addresses embedded with v2 into the v1 index, guard off
	mixed := Build("v1", factsV1)
	mixed.guard = false
	for _, v := range []Vector{{"n1", "v2", 600, 110}, {"n2", "v2", 640, 170}, {"n3", "v2", 560, 180}} {
		mixed.Add(v)
	}
	res := mixed.Nearest(q, 4)
	bad := 0
	for _, v := range res {
		if v.Model != q.Model {
			bad++
		}
	}
	fmt.Printf("mixed index, nearest 4:%s  -> %d of 4 are v2, no error raised\n", names(res), bad)

	// with the guard on, the mixed write is refused
	if err := v1.Add(Vector{"n1", "v2", 600, 110}); err != nil {
		fmt.Println("guard:", err)
	}

	// the fix: build v2 beside v1 from all facts, then switch readers
	factsV2 := map[string][2]float64{"a0": {730, 200}, "scam": {760, 90}, "a2": {800, 240},
		"a3": {840, 130}, "a4": {720, 120}, "a5": {850, 220}, "a6": {780, 160}}
	v2 := Build("v2", factsV2)
	serving := v1
	serving = v2 // one pointer move, then drop v1
	fmt.Printf("serving index %s with %d vectors, v1 dropped\n", serving.Model, len(serving.Vecs))
}
