// A claim's life: WaitForFirstConsumer holds the volume until the scheduler
// has placed the pod, the CSI driver creates it there, and the class's
// reclaim policy decides what a deleted claim means.
package main

import "fmt"

type Class struct {
	Name          string
	WaitForPod    bool
	ReclaimPolicy string
}

type Volume struct {
	Node   string
	SizeTi int
	Status string
}

type Claim struct {
	Name   string
	SizeTi int
	Class  Class
	Volume *Volume
}

type Node struct {
	Name   string
	FreeTi int
}

// schedule picks the first node with room on its disk, then the driver
// creates the volume there and binds it.
func schedule(c *Claim, nodes []*Node) string {
	for _, n := range nodes {
		if n.FreeTi >= c.SizeTi {
			n.FreeTi -= c.SizeTi
			c.Volume = &Volume{n.Name, c.SizeTi, "Bound"} // the CSI driver
			return n.Name
		}
		fmt.Printf("  %s: %d TiB free, needs %d: skip\n", n.Name, n.FreeTi, c.SizeTi)
	}
	return ""
}

// deleteClaim applies the reclaim policy.
func deleteClaim(c *Claim) *Volume {
	v := c.Volume
	c.Volume = nil
	if c.Class.ReclaimPolicy == "Delete" {
		fmt.Println("  the driver deletes the volume: the data is gone")
		return nil
	}
	v.Status = "Released"
	fmt.Printf("  volume on %s stays, status %s\n", v.Node, v.Status)
	return v
}

func main() {
	for _, policy := range []string{"Retain", "Delete"} {
		fmt.Println("reclaimPolicy", policy)
		class := Class{"local-nvme", true, policy}
		c := &Claim{"graph-data", 4, class, nil}
		fmt.Println("  claim created: volume", c.Volume, "(Pending, waits for a pod)")
		nodes := []*Node{{"node-1", 1}, {"node-2", 6}}
		fmt.Println("  pod placed on", schedule(c, nodes))
		v := deleteClaim(c)
		if v != nil {
			c2 := &Claim{"graph-data", 4, class, v}
			v.Status = "Bound"
			fmt.Printf("  an admin binds a new claim %s: status %s\n", c2.Name, c2.Volume.Status)
		}
	}
}
