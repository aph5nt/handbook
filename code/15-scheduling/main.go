// The scheduler places five pods on three nodes. Filter first: taints,
// required node affinity, pod anti-affinity. Then score: fewest pods wins,
// and the first node in the list breaks a tie.
package main

import "fmt"

type Node struct {
	Name   string
	Labels map[string]string
	Taint  string // key=value that pods must tolerate, "" for none
	Pods   []*Pod
	Up     bool
}

type Pod struct {
	Name         string
	Tolerates    string            // a taint this pod tolerates
	NodeAffinity map[string]string // required node labels
	AntiAffinity string            // never beside a pod of this app
	App          string
}

func feasible(p *Pod, n *Node) (bool, string) {
	if n.Taint != "" && n.Taint != p.Tolerates {
		return false, "taint " + n.Taint + " not tolerated"
	}
	for k, v := range p.NodeAffinity {
		if n.Labels[k] != v {
			return false, "node affinity " + k + "=" + v + " not met"
		}
	}
	for _, q := range n.Pods {
		if p.AntiAffinity != "" && q.App == p.AntiAffinity {
			return false, "anti-affinity: " + q.Name + " is here"
		}
	}
	return true, ""
}

func schedule(p *Pod, nodes []*Node) {
	var best *Node
	for _, n := range nodes {
		if ok, why := feasible(p, n); !ok {
			fmt.Printf("    %-7s filtered: %s\n", n.Name, why)
			continue
		}
		if best == nil || len(n.Pods) < len(best.Pods) { // score: fewest pods
			best = n
		}
	}
	if best == nil {
		fmt.Printf("  %s: Pending, no node fits\n", p.Name)
		return
	}
	best.Pods = append(best.Pods, p)
	fmt.Printf("  %s -> %s\n", p.Name, best.Name)
}

func main() {
	nodes := []*Node{
		{Name: "nvme-1", Labels: map[string]string{"disktype": "nvme"}, Up: true},
		{Name: "nvme-2", Labels: map[string]string{"disktype": "nvme"}, Up: true},
		{Name: "gpu-1", Labels: map[string]string{"gpu": "true"}, Taint: "gpu=true", Up: true},
	}
	api := &Pod{Name: "API pod", App: "api"}
	trainBad := &Pod{Name: "training (toleration only)", Tolerates: "gpu=true"}
	train := &Pod{Name: "training (toleration + affinity)", Tolerates: "gpu=true", NodeAffinity: map[string]string{"gpu": "true"}}
	be := func(i int) *Pod {
		return &Pod{Name: fmt.Sprintf("be-%d", i), App: "warehouse", AntiAffinity: "warehouse",
			NodeAffinity: map[string]string{"disktype": "nvme"}}
	}
	schedule(api, nodes)
	schedule(trainBad, nodes)
	nodes[1].Pods = nil // undo: replay with the fixed pod
	schedule(train, nodes)
	schedule(be(0), nodes)
	schedule(be(1), nodes)
	schedule(be(2), nodes)
	fmt.Println("nvme-1 loses power:")
	for _, p := range nodes[0].Pods {
		fmt.Printf("  %s down\n", p.Name)
	}
	for _, p := range nodes[1].Pods {
		fmt.Printf("  %s on nvme-2 keeps serving\n", p.Name)
	}
}
