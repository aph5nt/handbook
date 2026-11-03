// A tiny GitOps controller: it renders a chart from the values in git and
// makes the cluster match. Then the same hand scale without a controller,
// where the next helm upgrade silently undoes it.
package main

import (
	"bytes"
	"fmt"
	"text/template"
)

const chart = "replicas: {{.Replicas}}\nimage: registry.local/api@{{.Digest}}\n"

type Values struct {
	Replicas int
	Digest   string
}

// Commit is one commit on main: a message and the values file it holds.
type Commit struct {
	Msg    string
	Values Values
}

// render is what helm template does.
func render(v Values) string {
	var b bytes.Buffer
	template.Must(template.New("chart").Parse(chart)).Execute(&b, v)
	return b.String()
}

// Cluster holds the live object and the fields a hand command can change.
type Cluster struct {
	Replicas int
	Digest   string
}

func (c *Cluster) rendered() string { return render(Values{c.Replicas, c.Digest}) }

// reconcile is one pass of the controller: pull, render, compare, apply.
func reconcile(mainBranch []Commit, c *Cluster) {
	head := mainBranch[len(mainBranch)-1] // nobody pushes into the cluster
	desired := render(head.Values)
	if desired == c.rendered() {
		fmt.Printf("  in sync with %q: replicas %d\n", head.Msg, c.Replicas)
		return
	}
	before := c.Replicas
	c.Replicas, c.Digest = head.Values.Replicas, head.Values.Digest // apply
	fmt.Printf("  drift against %q: replicas %d -> %d\n", head.Msg, before, c.Replicas)
}

// helmUpgrade is the hand path: it writes whatever the values file says.
func helmUpgrade(v Values, c *Cluster) {
	fmt.Printf("  helm upgrade: replicas %d -> %d\n", c.Replicas, v.Replicas)
	c.Replicas, c.Digest = v.Replicas, v.Digest
}

func main() {
	fmt.Println("With a GitOps controller:")
	main1 := []Commit{{"replicas 1", Values{1, "sha256:7ab3"}}}
	c := &Cluster{1, "sha256:7ab3"}
	reconcile(main1, c)
	main1 = append(main1, Commit{"replicas 2 (reviewed)", Values{2, "sha256:7ab3"}})
	reconcile(main1, c)
	c.Replicas = 4 // kubectl scale on a busy night
	fmt.Println("  kubectl scale: replicas 4, git still says 2")
	reconcile(main1, c)
	main1 = append(main1, Commit{"revert: replicas 1", main1[0].Values})
	reconcile(main1, c)

	fmt.Println("By hand, no controller:")
	values := Values{1, "sha256:7ab3"}
	h := &Cluster{1, "sha256:7ab3"}
	h.Replicas = 4
	fmt.Println("  kubectl scale: replicas 4, values file still says 1")
	values.Digest = "sha256:c410" // next day, an unrelated new image
	helmUpgrade(values, h)
	fmt.Println("The fix: values file says 4, in a pull request")
	values.Replicas = 4
	helmUpgrade(values, h)
}
