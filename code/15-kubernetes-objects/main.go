// Kubernetes objects: the ReplicaSet loop, the Service endpoints, the
// kubelet's two probes, and a rolling update with maxSurge 1 and
// maxUnavailable 0.
package main

import (
	"fmt"
	"strings"
)

type Pod struct {
	Name  string
	Alive bool
	Ready bool // readiness probe passed
}

// reconcile is one pass of the ReplicaSet controller.
func reconcile(want int, pods []*Pod, newName func() string) []*Pod {
	var live []*Pod
	for _, p := range pods {
		if p.Alive {
			live = append(live, p)
		}
	}
	for len(live) < want { // too few: start more
		live = append(live, &Pod{Name: newName(), Alive: true})
	}
	for len(live) > want { // too many: delete the newest
		live = live[:len(live)-1]
	}
	return live
}

// endpoints is what the Service sends traffic to.
func endpoints(pods []*Pod) []string {
	var out []string
	for _, p := range pods {
		if p.Alive && p.Ready {
			out = append(out, p.Name)
		}
	}
	return out
}

// probe is the kubelet. Liveness restarts, readiness only removes.
func probe(p *Pod, livenessOK, readinessOK bool) {
	if !livenessOK {
		p.Ready = false // restarted: starts again, not ready
		return
	}
	p.Ready = readinessOK
}

type RS struct{ Pods, Ready int }

// rollStep is one pass of the Deployment controller during a roll.
func rollStep(replicas, maxSurge, maxUnavailable int, old, nw *RS) string {
	if nw.Pods < replicas && old.Pods+nw.Pods < replicas+maxSurge {
		nw.Pods++ // start one surge pod
		return "start a v2 pod"
	}
	if old.Pods > 0 && old.Pods+nw.Ready > replicas-maxUnavailable {
		old.Pods-- // only after a new pod is ready
		return "stop a v1 pod"
	}
	return "wait for readiness"
}

func roll(healthy, hasProbe bool) {
	old, nw := &RS{Pods: 3, Ready: 3}, &RS{}
	for pass := 1; pass <= 8 && old.Pods > 0; pass++ {
		did := rollStep(3, 1, 0, old, nw)
		if !hasProbe || healthy { // no probe: started counts as ready
			nw.Ready = nw.Pods
		}
		fmt.Printf("    pass %d: %-18s v1 %d, v2 %d (ready %d)\n", pass, did, old.Pods, nw.Pods, nw.Ready)
	}
}

func main() {
	n := 0
	name := func() string { n++; return fmt.Sprintf("api-%c", 'a'+n-1) }
	pods := reconcile(3, nil, name)
	fmt.Println("start: wants 3, started", len(pods), " endpoints", endpoints(pods))
	for _, p := range pods {
		probe(p, true, true)
	}
	fmt.Println("readiness passes: endpoints", strings.Join(endpoints(pods), " "))
	pods[1].Alive = false // 03:00, the node under api-b loses power
	fmt.Println("api-b dead: endpoints", strings.Join(endpoints(pods), " "))
	pods = reconcile(3, pods, name)
	fmt.Println("reconcile: wants 3, has 2, starts", pods[2].Name, " endpoints", strings.Join(endpoints(pods), " "))
	probe(pods[2], true, true)
	fmt.Println("api-d ready: endpoints", strings.Join(endpoints(pods), " "))
	for _, p := range pods { // liveness checks the database, which pauses 5 s
		probe(p, false, false)
	}
	fmt.Println("liveness checks the database: endpoints", len(endpoints(pods)))

	fmt.Println("Roll v1 -> v2, maxSurge 1, maxUnavailable 0:")
	roll(true, true)
	fmt.Println("Broken v2, with a readiness probe:")
	roll(false, true)
	fmt.Println("Broken v2, no readiness probe:")
	roll(false, false)
}
