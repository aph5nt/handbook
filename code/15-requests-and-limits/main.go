// Requests and limits: the scheduler packs by requests, the kernel
// enforces limits. CPU over the limit is slowed, memory over the limit
// is killed, and a node that runs out kills a pod it picks.
package main

import (
	"fmt"
	"sort"
)

type Pod struct {
	Name           string
	ReqGi, LimGi   float64 // memory request and limit
	ReqCPU, LimCPU float64
}

// fits is the scheduler: requests only, limits are ignored.
func fits(node []Pod, capGi float64, p Pod) bool {
	sum := p.ReqGi
	for _, q := range node {
		sum += q.ReqGi
	}
	return sum <= capGi
}

// cpuGranted is the CFS quota: over the limit, the pod waits.
func cpuGranted(want, limCPU float64) (got float64, throttled bool) {
	if want > limCPU {
		return limCPU, true
	}
	return want, false
}

// memCheck is the kernel: over the limit, the pod is killed.
func memCheck(useGi, limGi float64) string {
	if useGi > limGi {
		return "OOMKilled"
	}
	return "ok"
}

// nodeOOM picks a victim when the node itself runs out.
func nodeOOM(use map[string]float64, capGi float64) string {
	names := []string{}
	for n := range use {
		names = append(names, n)
	}
	sort.Strings(names)
	total, victim := 0.0, ""
	for _, n := range names {
		total += use[n]
		if victim == "" || use[n] > use[victim] {
			victim = n
		}
	}
	if total <= capGi {
		return ""
	}
	return victim // the biggest user, inside its limit or not
}

func place(pods []Pod, capGi float64) []Pod {
	var node []Pod
	for _, p := range pods {
		if fits(node, capGi, p) {
			node = append(node, p)
		} else {
			fmt.Printf("  %s: Pending, no room\n", p.Name)
		}
	}
	return node
}

func main() {
	const nodeGi = 64
	gs := Pod{"graph sync", 16, 16, 2, 4}
	api := Pod{"api", 2, 2, 1, 2}
	node := place([]Pod{gs, api}, nodeGi)
	fmt.Printf("Tab 1: %d pods placed on a 64 GiB node\n", len(node))
	got, thr := cpuGranted(2.6, api.LimCPU)
	fmt.Printf("  api wants 2.6 CPU, gets %.1f, throttled %v\n", got, thr)
	for _, use := range []float64{15, 21} {
		fmt.Printf("  graph sync at %.0fGi of 16Gi: %s\n", use, memCheck(use, gs.LimGi))
	}
	gs.ReqGi, gs.LimGi = 32, 32 // the measured peak was 21Gi
	fmt.Printf("  limit 32Gi, the 21Gi page: %s\n", memCheck(21, gs.LimGi))

	fmt.Println("Tab 2: five pods, request 4Gi, limit 24Gi")
	var five []Pod
	for i := 1; i <= 5; i++ {
		five = append(five, Pod{Name: fmt.Sprintf("pod %d", i), ReqGi: 4, LimGi: 24})
	}
	node = place(five, nodeGi)
	fmt.Printf("  placed %d, reserved %.0fGi, limits add up to %.0fGi\n", len(node), 4.0*float64(len(node)), 24.0*float64(len(node)))
	use := map[string]float64{"pod 1": 13, "pod 2": 13, "pod 3": 13.4, "pod 4": 12.8, "pod 5": 12.8}
	fmt.Println("  load: 65Gi in use, every pod inside its 24Gi limit")
	fmt.Println("  node OOM killer picks:", nodeOOM(use, nodeGi))
	for i := range five {
		five[i].ReqGi, five[i].LimGi = 14, 14 // measured peak
	}
	node = place(five, nodeGi)
	fmt.Printf("  request = limit 14Gi: placed %d, reserved %.0fGi\n", len(node), 14.0*float64(len(node)))
}
