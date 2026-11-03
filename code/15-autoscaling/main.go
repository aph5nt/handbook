// The HPA formula and the cluster autoscaler on the query API story, then
// the two traps: autoscaling a single writer, and an HPA and a VPA on one
// metric.
package main

import (
	"fmt"
	"math"
)

const targetCPU = 60.0 // percent of the request

// desired is the HPA formula.
func desired(current int, metric float64) int {
	return int(math.Ceil(float64(current) * metric / targetCPU))
}

// stabilised is the scale-down rule: use the highest desired count
// seen in the last 5 minutes, so a short dip keeps its pods.
func stabilised(recent []int) int {
	top := 0
	for _, d := range recent {
		top = max(top, d)
	}
	return top
}

// Cluster counts free pod slots. A Pending pod that a new node could hold
// makes the cluster autoscaler add a node.
type Cluster struct {
	Free, Nodes, SlotsPerNewNode int
}

// place returns how many of the wanted pods stay Pending.
func (c *Cluster) place(pods int) (pending int) {
	if pods > c.Free {
		pending = pods - c.Free
	}
	c.Free -= pods - pending
	return pending
}

// the cluster autoscaler: a Pending pod makes it add a node.
func (c *Cluster) addNode() {
	c.Nodes++
	c.Free += c.SlotsPerNewNode
}

// removeNode: a node idle for 10 minutes is removed.
func (c *Cluster) removeNode() {
	c.Nodes--
	c.Free -= c.SlotsPerNewNode
}

// percent is the HPA metric: use as a share of the request. A VPA moves the request.
func percent(usedCores, requestCores float64) float64 { return 100 * usedCores / requestCores }

// clones is what an HPA does to a writer: every pod starts at the one checkpoint.
func clones(pods, checkpoint int) []int {
	starts := make([]int, pods)
	for i := range starts {
		starts[i] = checkpoint // same image, same checkpoint: the same blocks twice
	}
	return starts
}

// ranges is how a writer scales: one Job per block range, each its own checkpoint.
func ranges(from, to, jobs int) [][2]int {
	size := (to - from + 1) / jobs
	var out [][2]int
	for j := 0; j < jobs; j++ {
		out = append(out, [2]int{from + j*size, from + (j+1)*size - 1})
	}
	return out
}

func main() {
	load := 255.0 // total CPU percent across the pods: 3 pods at 85
	replicas := 3
	fmt.Printf("3 pods at 60%%: desired %d\n", desired(3, 60))
	want := desired(replicas, load/float64(replicas))
	fmt.Printf("3 pods at 85%%: 3 x 85 / 60 = %.2f, ceil = %d\n", 3*85/targetCPU, want)

	c := &Cluster{Free: 1, Nodes: 2, SlotsPerNewNode: 3}
	pending := c.place(want - replicas)
	running := want - pending
	fmt.Printf("wants %d: %d run, %d Pending, CPU per pod %.0f%%\n", want, running, pending, load/float64(running))
	if pending > 0 {
		c.addNode()
		fmt.Printf("Pending pod fits a new node: nodes %d\n", c.Nodes)
		pending = c.place(pending)
	}
	replicas = want
	fmt.Printf("%d pods run, %d Pending, CPU per pod %.0f%%, desired stays %d\n",
		replicas, pending, load/float64(replicas), desired(replicas, load/float64(replicas)))

	load = 174 // the news cycle ends
	metric := load / float64(replicas)
	want = desired(replicas, metric)
	fmt.Printf("traffic falls: 5 x %.1f / 60 -> %d\n", metric, want)
	fmt.Printf("  a 1-minute dip, window holds 5 and 3: stays %d\n", stabilised([]int{5, 5, 5, 5, want}))
	replicas = stabilised([]int{want, want, want, want, want})
	fmt.Printf("  5 minutes of %d: scales to %d\n", want, replicas)
	c.removeNode()
	fmt.Printf("%d pods at %.0f%%, node 3 idle for 10 minutes: removed, nodes %d\n", replicas, load/float64(replicas), c.Nodes)

	fmt.Println("Trap 1: HPA on a single writer")
	checkpoint := 9506000
	fmt.Printf("  1 pod at 95%%: desired %d\n", desired(1, 95))
	fmt.Printf("  the pods start at %v: both write the same blocks\n", clones(desired(1, 95), checkpoint))
	fmt.Printf("  the fix, one Job per range: %v\n", ranges(9506000, 9905999, 4))

	fmt.Println("Trap 2: VPA and HPA on one metric")
	used, request, pods := 1.2, 1.0, 3 // CPU cores per pod, request, pods
	for round := 1; round <= 3; round++ {
		total := used * float64(pods)
		pct := percent(used, request)
		pods = desired(pods, pct)
		used = total / float64(pods)
		fmt.Printf("  round %d: %.0f%% of request, HPA sets %d pods", round, pct, pods)
		request = used * 100 / 40 // VPA: wants 40% use, raises or cuts the request
		fmt.Printf(", VPA sets request %.2f\n", request)
		used = total / float64(pods)
	}
}
