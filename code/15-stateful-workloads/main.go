// A StatefulSet: pods start in order, each with a stable name and its own
// claim. A local volume ties the pod to one node. A replicated one does not.
package main

import "fmt"

type Claim struct {
	Name, Class, Node string
	Replicas          []string // nodes holding a copy, for a replicated class
}

type Pod struct {
	Name  string
	Claim *Claim
	Node  string
	Ready bool
}

// startOrdered is the StatefulSet controller: pod N waits for pod N-1.
func startOrdered(n int, claims map[string]*Claim, nodes []string) []*Pod {
	var pods []*Pod
	for i := 0; i < n; i++ {
		if i > 0 && !pods[i-1].Ready {
			fmt.Printf("  be-%d waits: be-%d is not ready\n", i, i-1)
			return pods
		}
		name := fmt.Sprintf("be-%d", i)
		c, ok := claims["data-"+name]
		if !ok { // from volumeClaimTemplates, named after the pod
			c = &Claim{Name: "data-" + name, Class: "local-nvme", Node: nodes[i]}
			claims[c.Name] = c
			fmt.Printf("  created claim %s on %s\n", c.Name, c.Node)
		}
		p := &Pod{Name: name, Claim: c, Node: c.Node, Ready: true}
		pods = append(pods, p)
		fmt.Printf("  %s started on %s, bound to %s\n", p.Name, p.Node, c.Name)
	}
	return pods
}

// place finds a node for a pod whose node died.
func place(c *Claim, up map[string]bool) string {
	if c.Class == "local-nvme" { // the data is on that one disk
		if up[c.Node] {
			return c.Node
		}
		return "Pending: its data is only on " + c.Node
	}
	for _, n := range c.Replicas {
		if up[n] {
			return n
		}
	}
	return "Pending: no copy on a live node"
}

// drain evicts a pod only if the budget stays met.
func mayEvict(ready, minAvailable int) bool { return ready-1 >= minAvailable }

func main() {
	claims := map[string]*Claim{}
	nodes := []string{"node-1", "node-2"}
	fmt.Println("First start:")
	pods := startOrdered(2, claims, nodes)
	fmt.Println("node-1 reboots, be-0 comes back:")
	pods = startOrdered(1, claims, nodes)
	fmt.Printf("  same name %s, same claim %s\n", pods[0].Name, pods[0].Claim.Name)

	fmt.Println("Drain, PodDisruptionBudget minAvailable 1:")
	fmt.Println("  2 ready, evict be-0:", mayEvict(2, 1))
	fmt.Println("  1 ready, evict be-1:", mayEvict(1, 1))

	up := map[string]bool{"node-2": true, "node-3": true}
	fe := &Claim{Name: "meta-fe-0", Class: "longhorn", Replicas: []string{"node-1", "node-2", "node-3"}}
	fmt.Println("node-1 dies for good:")
	fmt.Println("  be-0 (local-nvme):", place(claims["data-be-0"], up))
	fmt.Println("  fe-0 (longhorn, 3 copies):", place(fe, up))
	up["node-1"] = true
	fmt.Println("node-1 returns: be-0:", place(claims["data-be-0"], up))
}
