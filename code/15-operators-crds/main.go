// An operator: a control loop that knows one product. It reads a
// StarRocksCluster object and makes the pods match, one safe step per pass.
package main

import "fmt"

type Spec struct {
	Backends int
	Version  string
}

type Status struct {
	Ready   bool
	Version string
}

// Cluster is the custom resource: what you want, and what is true.
type Cluster struct {
	Name   string
	Spec   Spec
	Status Status
}

// Pods is what the operator owns, by pod name.
type Pods map[string]string // name -> version

// Reconcile takes one safe step and returns true when nothing is left.
func Reconcile(c *Cluster, pods Pods) (string, bool) {
	if _, ok := pods["fe-0"]; !ok {
		pods["fe-0"] = c.Spec.Version
		return "create fe-0, wait until ready", false
	}
	for i := 0; i < c.Spec.Backends; i++ { // create in order, one at a time
		name := fmt.Sprintf("be-%d", i)
		if _, ok := pods[name]; !ok {
			pods[name] = c.Spec.Version
			return "create " + name + ", wait until it joins", false
		}
	}
	for i := c.Spec.Backends - 1; i >= 0; i-- { // upgrade highest first
		name := fmt.Sprintf("be-%d", i)
		if pods[name] != c.Spec.Version {
			pods[name] = c.Spec.Version
			return "upgrade " + name + ", wait until caught up", false
		}
	}
	if pods["fe-0"] != c.Spec.Version { // the frontend goes last
		pods["fe-0"] = c.Spec.Version
		return "upgrade fe-0", false
	}
	c.Status = Status{true, c.Spec.Version}
	return "status: ready, version " + c.Spec.Version, true
}

// deleteCRD is the cascade: every object of the kind goes, then all they own.
func deleteCRD(objects map[string]*Cluster, pods Pods) int {
	for name := range objects {
		delete(objects, name)
	}
	n := len(pods)
	for p := range pods {
		delete(pods, p) // each pod has an owner reference to its object
	}
	return n
}

func run(c *Cluster, pods Pods) {
	for {
		step, done := Reconcile(c, pods)
		fmt.Println(" ", step)
		if done {
			return
		}
	}
}

func main() {
	prod := &Cluster{Name: "prod", Spec: Spec{Backends: 3, Version: "3.2"}}
	pods := Pods{}
	fmt.Println("kubectl apply prod, 3 backends, version 3.2:")
	run(prod, pods)
	fmt.Println("version changes to 3.3 in git:")
	prod.Spec.Version = "3.3"
	run(prod, pods)

	fmt.Println("a cleanup deletes the CRD StarRocksCluster:")
	objects := map[string]*Cluster{"prod": prod}
	removed := deleteCRD(objects, pods)
	fmt.Printf("  objects left: %d, owned pods removed: %d\n", len(objects), removed)
}
