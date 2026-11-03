// CI runners: the same three jobs on a hosted runner and a self-hosted one,
// counted in downloads, and the rule that keeps a fork's code off your host.
package main

import "fmt"

type Runner struct {
	Name       string
	SelfHosted bool
	Ephemeral  bool
	Baked      []string // caches baked into the disk the runner starts from
	cache      map[string]bool
}

// run needs the image and the modules. A hit needs no download.
func (r *Runner) run(job string, needs []string) (downloads int) {
	if !r.SelfHosted || r.Ephemeral { // a fresh machine, every cache empty
		r.cache = map[string]bool{}
		for _, b := range r.Baked {
			r.cache[b] = true
		}
	}
	for _, n := range needs {
		if !r.cache[n] {
			r.cache[n] = true
			downloads++
		}
	}
	return downloads
}

type Repo struct {
	Name    string
	Private bool
}

type PR struct {
	Repo     Repo
	FromFork bool
}

// mayRun is the rule: a fork's code never reaches your own machine.
func mayRun(pr PR, r *Runner) (bool, string) {
	if !r.SelfHosted {
		return true, "thrown away after the job"
	}
	if !pr.Repo.Private {
		return false, "self-hosted runners serve private repositories only"
	}
	if pr.FromFork {
		return false, "a fork's code does not run on your host"
	}
	return true, "private repository, own branch"
}

func main() {
	needs := []string{"starrocks image", "go modules", "chain recording"}
	jobs := []string{"t1", "t2-a", "t2-b"}
	for _, r := range []*Runner{
		{Name: "hosted"},
		{Name: "self-hosted", SelfHosted: true, cache: map[string]bool{}},
		{Name: "self-hosted, ephemeral, caches baked into its disk", SelfHosted: true, Ephemeral: true, Baked: needs},
	} {
		total := 0
		for _, j := range jobs {
			total += r.run(j, needs)
		}
		fmt.Printf("%s: %d downloads for %d jobs\n", r.Name, total, len(jobs))
	}
	own := &Runner{Name: "build host", SelfHosted: true}
	for _, pr := range []PR{
		{Repo{"indexer", true}, false},
		{Repo{"public-sdk", false}, true},
		{Repo{"public-sdk", false}, false},
	} {
		ok, why := mayRun(pr, own)
		fmt.Printf("repo %s, fork %v: run %v (%s)\n", pr.Repo.Name, pr.FromFork, ok, why)
	}
}
