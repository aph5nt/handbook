// Latency percentiles and queueing: the story of the lesson
// 12-latency-percentiles-and-queueing.html. A hundred warehouse queries: the
// average describes nobody, p50 is the typical user, p99 is the tail. Then
// one server's wait grows like u / (1 - u) as it gets busier.
package main

import (
	"fmt"
	"math"
	"sort"
)

// Times are the same hundred query times the lesson draws, in ms.
func Times() []int {
	var t []int
	for k := 0; k < 90; k++ {
		t = append(t, 30+(k*37)%61) // the crowd: 30 to 90 ms
	}
	for k := 0; k < 8; k++ {
		t = append(t, 150+k*60) // a slow group: 150 to 570 ms
	}
	return append(t, 2400, 3800) // two very slow ones
}

func Average(t []int) int {
	sum := 0
	for _, v := range t {
		sum += v
	}
	return int(math.Round(float64(sum) / float64(len(t))))
}

// Percentile sorts and takes the value at rank p of n (p50 = the 50th).
func Percentile(t []int, p int) int {
	s := append([]int(nil), t...)
	sort.Ints(s)
	return s[len(s)*p/100-1]
}

// Wait is the average queue wait of one server, in service times, at
// utilisation u: u / (1 - u).
func Wait(u float64) float64 { return u / (1 - u) }

func main() {
	t := Times()
	fmt.Printf("average %d ms, p50 %d ms, p99 %d ms\n", Average(t), Percentile(t, 50), Percentile(t, 99))
	over := 0
	for _, v := range t {
		if v > 2000 {
			over++
		}
	}
	fmt.Printf("agent timeout 2 s: %d requests failed while the average says %d ms\n", over, Average(t))
	allFast := math.Pow(0.99, 100)
	fmt.Printf("fan-out to 100 backends: all fast %.0f%%, at least one slow %.0f%%\n", allFast*100, (1-allFast)*100)

	fmt.Println("== Queueing")
	for _, u := range []float64{0.5, 0.7, 0.8, 0.9, 0.95} {
		fmt.Printf("%.0f%% busy: wait %.1f service times\n", u*100, Wait(u))
	}
}
