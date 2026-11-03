// Replays the sidecar lesson: a price oracle Job in a pod with a log
// sidecar, an ambassador that retries outbound calls and a metrics adapter.
// Then the pod that never finishes, and native sidecars that fix it.
package main

import (
	"fmt"
	"strings"
)

// Pod holds the containers. They share one network and one folder.
type Pod struct {
	shared  map[string][]string // the shared folder: file -> lines
	running map[string]bool     // container -> still running
	native  map[string]bool     // helper declared as a native sidecar (restartPolicy Always)
	exit    int                 // the main container's exit code
}

// priceAPI is outside the pod. It fails the first call, then answers.
type priceAPI struct{ calls int }

func (p *priceAPI) get() (int, []float64) {
	p.calls++
	if p.calls == 1 {
		return 503, nil
	}
	prices := make([]float64, 48) // one price per token
	for i := range prices {
		prices[i] = 1 + float64(i)/100
	}
	return 200, prices
}

// ambassador is the local proxy on localhost:9000: retries, at most 3 tries.
func ambassador(p *Pod, api *priceAPI) []float64 {
	for try := 1; try <= 3; try++ {
		code, body := api.get()
		p.shared["log"] = append(p.shared["log"], fmt.Sprintf("ambassador: try %d -> %d", try, code))
		if code == 200 {
			return body
		}
	}
	return nil
}

// mainJob knows only localhost. It writes rows, a log and a status file.
func mainJob(p *Pod, api *priceAPI) {
	prices := ambassador(p, api) // a call to localhost:9000
	p.shared["log"] = append(p.shared["log"], fmt.Sprintf("fetched %d prices", len(prices)))
	p.shared["log"] = append(p.shared["log"], fmt.Sprintf("wrote %d rows", len(prices)))
	p.shared["status"] = []string{fmt.Sprintf("rows=%d", len(prices)), "errors=0"}
	p.running["main"] = false
	p.exit = 0
}

// sidecar ships every log line in the shared folder to Loki.
func sidecar(p *Pod, loki *[]string) {
	*loki = append(*loki, p.shared["log"]...)
}

// adapter answers a Prometheus scrape by reshaping the status file.
func adapter(p *Pod) string {
	var out []string
	for _, kv := range p.shared["status"] {
		k, v, _ := strings.Cut(kv, "=")
		out = append(out, "price_oracle_"+k+" "+v)
	}
	return strings.Join(out, "\n")
}

// mainExited is what the kubelet does when the main container stops.
func (p *Pod) mainExited() {
	for h := range p.running {
		if p.native[h] { // native sidecars are stopped after the main one
			p.running[h] = false
		}
	}
}

// jobComplete: the pod is done when every container has exited.
func (p *Pod) jobComplete() (bool, []string) {
	var alive []string
	for _, c := range []string{"main", "sidecar", "ambassador", "adapter"} {
		if p.running[c] {
			alive = append(alive, c)
		}
	}
	return len(alive) == 0 && p.exit == 0, alive
}

func run(native bool) {
	p := &Pod{
		shared:  map[string][]string{"log": {}},
		running: map[string]bool{"main": true, "sidecar": true, "ambassador": true, "adapter": true},
		native:  map[string]bool{"sidecar": native, "ambassador": native, "adapter": native},
	}
	api := &priceAPI{}
	var loki []string
	mainJob(p, api)
	sidecar(p, &loki)
	if !native {
		fmt.Printf("  ambassador: price API calls %d, %s\n", api.calls, strings.Join(p.shared["log"][:2], ", "))
		fmt.Printf("  sidecar: lines shipped to Loki %d\n", len(loki))
		fmt.Printf("  adapter on scrape:\n    %s\n", strings.ReplaceAll(adapter(p), "\n", "\n    "))
		fmt.Printf("  main exited %d\n", p.exit)
	}
	p.mainExited()
	done, alive := p.jobComplete()
	fmt.Printf("  still running: %v, Job complete: %v\n", alive, done)
}

func main() {
	fmt.Println("helpers as plain containers:")
	run(false)
	fmt.Println("helpers as native sidecars (Kubernetes 1.29 and later):")
	run(true)
}
