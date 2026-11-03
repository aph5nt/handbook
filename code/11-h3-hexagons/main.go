// Command h3 replays the hexagon lesson with a small axial hexagon grid.
// It is a simplified model of the idea, not the real H3 library: the point
// is the single neighbour distance, the smoothing, and the inexact roll-up.
package main

import (
	"fmt"
	"math"
)

type Cell struct{ Q, R int }

func CellOf(x, y, size, rot float64) Cell {
	c, s := math.Cos(rot), math.Sin(rot)
	x, y = x*c+y*s, -x*s+y*c
	q := (math.Sqrt(3)/3*x - y/3) / size
	r := (2.0 / 3 * y) / size
	return round(q, r)
}

func Ring(c Cell) []Cell {
	d := []Cell{{1, 0}, {1, -1}, {0, -1}, {-1, 0}, {-1, 1}, {0, 1}}
	out := make([]Cell, 6)
	for i, v := range d {
		out[i] = Cell{c.Q + v.Q, c.R + v.R}
	}
	return out
}

func Smooth(v map[Cell]float64, c Cell) float64 {
	sum := v[c]
	for _, n := range Ring(c) {
		sum += v[n]
	}
	return sum / 7
}

func Surge(req, drv map[Cell]float64, c Cell) bool {
	return Smooth(req, c)/Smooth(drv, c) > 2
}

func Hint(req, drv map[Cell]float64, c Cell) (Cell, bool) {
	if drv[c] <= req[c] {
		return c, false
	}
	best, top := c, 0.0
	for _, n := range Ring(c) {
		if drv[n] > 0 && req[n]/drv[n] > top {
			best, top = n, req[n]/drv[n]
		}
	}
	return best, true
}

func Count(m map[Cell]float64, c Cell) { m[c]++ }

func Parent(c Cell, size, big, rot float64) Cell {
	x, y := centre(c, size)
	return CellOf(x, y, big, rot)
}

// SquareDist: a side neighbour is 1.0 away, a corner one about 1.41.
func SquareDist(dx, dy float64) float64 { return math.Hypot(dx, dy) }

func EqualWeight(own float64, near []float64) float64 {
	sum := own
	for _, v := range near {
		sum += v
	}
	return sum / float64(len(near)+1)
}

func round(q, r float64) Cell {
	s := -q - r
	rq, rr, rs := math.Round(q), math.Round(r), math.Round(s)
	dq, dr, ds := math.Abs(rq-q), math.Abs(rr-r), math.Abs(rs-s)
	if dq > dr && dq > ds {
		rq = -rr - rs
	} else if dr > ds {
		rr = -rq - rs
	}
	return Cell{int(rq), int(rr)}
}

func centre(c Cell, size float64) (float64, float64) {
	return size * math.Sqrt(3) * (float64(c.Q) + float64(c.R)/2), size * 1.5 * float64(c.R)
}

func main() {
	fmt.Println("hex tab")
	o := Cell{0, 0}
	req, drv := map[Cell]float64{}, map[Cell]float64{}
	for i := 0; i < 40; i++ {
		Count(req, o)
	}
	for i := 0; i < 3; i++ {
		Count(drv, o)
	}
	// ring order: east, north-east, north-west, west, south-west, south-east
	rq := []float64{15, 9, 12, 3, 6, 11}
	dr := []float64{1, 2, 4, 9, 2, 3}
	ring := Ring(o)
	for i, n := range ring {
		req[n], drv[n] = rq[i], dr[i]
	}
	fmt.Printf("step 2: a point at (120, 40), size 500 -> cell %v\n", CellOf(120, 40, 500, 0))
	fmt.Println("step 3: centre 40 requests / 3 drivers, west", req[ring[3]], "/", drv[ring[3]])
	fmt.Printf("step 4: smoothed requests (40+12+9+15+3+6+11)/7 = %.1f, drivers %.1f\n", Smooth(req, o), Smooth(drv, o))
	fmt.Printf("step 5: ratio %.1f > 2: surge %v\n", Smooth(req, o)/Smooth(drv, o), Surge(req, drv, o))
	to, ok := Hint(req, drv, ring[3])
	fmt.Printf("  west 9 drivers > 3 requests: hint %v, go to %v (the centre: %v)\n", ok, to, to == o)
	bad, total := 0, 0
	rot := math.Atan(math.Sqrt(3) / 5)
	big := 500 * math.Sqrt(7)
	for x := -3000.0; x < 3000; x += 23 {
		for y := -3000.0; y < 3000; y += 23 {
			total++
			if Parent(CellOf(x, y, 500, 0), 500, big, rot) != CellOf(x, y, big, rot) {
				bad++
			}
		}
	}
	fmt.Printf("step 6: roll-up by parent vs the point's real big cell: %d of %d points differ (%.0f%%)\n", bad, total, float64(bad)*100/float64(total))

	fmt.Println("sq tab")
	sides := 6.0 + 5 + 6 + 5
	corners := 2.0 + 30 + 2 + 2
	fmt.Println("step 2-3: side neighbours 1.00 away, corner neighbours", math.Round(math.Sqrt2*100)/100, "away")
	fmt.Printf("step 4: equal weights: (10+%v+%v)/9 = %.1f; with a quiet corner of 2 it would be %.1f\n",
		sides, corners, EqualWeight(10, []float64{6, 5, 6, 5, 2, 30, 2, 2}), EqualWeight(10, []float64{6, 5, 6, 5, 2, 2, 2, 2}))
	fmt.Println("step 5: hexagons: 6 neighbours, distance", math.Round(hexDist()*100)/100, "for all")
}

func hexDist() float64 {
	x, y := centre(Cell{1, 0}, 1/math.Sqrt(3))
	return math.Hypot(x, y)
}
