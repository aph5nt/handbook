// Two controllers from the workload kinds lesson.
//
//	DaemonSet: one log shipper per node, rolled one node at a time.
//	CronJob:   a nightly tiering Job, with and without concurrencyPolicy Forbid.
package main

import (
	"fmt"
	"sort"
)

// syncDaemonSet makes the shipper set match the node set: one pod per node.
func syncDaemonSet(nodes map[string]bool, pods map[string]string) {
	for _, n := range sorted(nodes) {
		if _, ok := pods[n]; !ok {
			pods[n] = "starting"
			fmt.Printf("  node %s has no shipper: create one\n", n)
		}
	}
	for n := range pods {
		if !nodes[n] {
			delete(pods, n)
			fmt.Printf("  node %s is gone: delete its shipper\n", n)
		}
	}
}

func sorted(m map[string]bool) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// rollOut replaces shippers with a leaking version, maxUnavailable at a
// time. With no memory limit the leak takes the whole node down. With a
// 200 MiB limit only the shipper is killed, and the roll stops at node 1.
func rollOut(nodes []string, maxUnavailable int, limitMiB int) {
	leakMiB := 4096 // the new version grows until something stops it
	for i := 0; i < len(nodes); i += maxUnavailable {
		end := min(i+maxUnavailable, len(nodes))
		batch := nodes[i:end]
		if limitMiB > 0 && leakMiB > limitMiB {
			fmt.Printf("  %v: shipper OOM-killed at %d MiB, node fine, new pod never ready: roll stops\n", batch, limitMiB)
			return
		}
		fmt.Printf("  %v: no limit, shipper reaches %d MiB, node out of memory\n", batch, leakMiB)
	}
}

// cron runs at each 02:00. Job d takes runHours[d] hours.
func cron(policy string, runHours []int) {
	type job struct{ day, end int }
	var active []job
	for day, h := range runHours {
		now := day*24 + 2
		still := active[:0]
		for _, j := range active {
			if j.end > now {
				still = append(still, j)
			}
		}
		active = still
		if len(active) > 0 && policy == "Forbid" {
			fmt.Printf("  day %d 02:00: job from day %d still active: skip\n", day, active[0].day)
			continue
		}
		if len(active) > 0 {
			fmt.Printf("  day %d 02:00: job from day %d still active: start another, %d jobs on the same partitions\n", day, active[0].day, len(active)+1)
		} else {
			fmt.Printf("  day %d 02:00: create a job, it needs %d h\n", day, h)
		}
		active = append(active, job{day, now + h})
	}
}

// runJob retries a failed pod up to backoffLimit times.
func runJob(exits []int, backoffLimit int) {
	for try, code := range exits {
		if code == 0 {
			fmt.Printf("  pod %d exit 0: job complete\n", try+1)
			return
		}
		if try >= backoffLimit {
			fmt.Println("  backoffLimit reached: job failed")
			return
		}
		fmt.Printf("  pod %d exit %d: retry %d of %d\n", try+1, code, try+1, backoffLimit)
	}
}

func main() {
	fmt.Println("DaemonSet:")
	nodes := map[string]bool{"node-1": true, "node-2": true}
	pods := map[string]string{}
	syncDaemonSet(nodes, pods)
	nodes["node-3"] = true // a node joins
	syncDaemonSet(nodes, pods)
	names := sorted(nodes)
	fmt.Printf("  %d nodes, %d shippers\n", len(nodes), len(pods))
	fmt.Println("Roll a leaking version, all at once, no limit:")
	rollOut(names, 3, 0)
	fmt.Println("Roll it with maxUnavailable 1 and a 200 MiB limit:")
	rollOut(names, 1, 200)

	fmt.Println("Job with backoffLimit 3:")
	runJob([]int{1, 0}, 3)
	fmt.Println("CronJob, concurrencyPolicy Allow (the default):")
	cron("Allow", []int{26, 3, 3})
	fmt.Println("CronJob, concurrencyPolicy Forbid:")
	cron("Forbid", []int{26, 3, 3})
}
