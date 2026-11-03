// Two lanes and a pointer. The pointer moves only when the idle lane passes
// the gate. A check diffs the two values files, so a fix on one lane cannot
// stay there. Rollback is a revert of the pointer commit.
package main

import (
	"fmt"
	"sort"
)

const (
	gateMinutes = 30  // healthy on live behaviour, no longer
	maxLag      = 100 // blocks behind the chain head
)

// Lane is one full copy of the stack.
type Lane struct {
	Name         string
	Digest       string
	HeadLag      int
	HealthyMin   int
	GoldenPassed int
	GoldenTotal  int
	Values       map[string]string
}

// gate decides if the idle lane may serve.
func gate(l Lane) error {
	switch {
	case l.HeadLag > maxLag:
		return fmt.Errorf("lane %s is %d blocks behind the head", l.Name, l.HeadLag)
	case l.HealthyMin < gateMinutes:
		return fmt.Errorf("lane %s healthy for %d of %d minutes", l.Name, l.HealthyMin, gateMinutes)
	case l.GoldenPassed < l.GoldenTotal:
		return fmt.Errorf("lane %s passed %d of %d golden checks", l.Name, l.GoldenPassed, l.GoldenTotal)
	}
	return nil
}

// flip is the pull request that moves the pointer.
func flip(pointer *string, idle Lane) error {
	if err := gate(idle); err != nil {
		return err
	}
	*pointer = idle.Name
	return nil
}

// drift returns every setting that differs, other than the allowed ones.
func drift(a, b map[string]string) []string {
	allowed := map[string]bool{"namespace": true, "host": true}
	keys := map[string]bool{}
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	var out []string
	for k := range keys {
		if a[k] != b[k] && !allowed[k] {
			out = append(out, fmt.Sprintf("%s: A %s, B %s", k, a[k], b[k]))
		}
	}
	sort.Strings(out)
	return out
}

func main() {
	pointer := "A"
	b := Lane{Name: "B", Digest: "@cd34", HeadLag: 400000, GoldenTotal: 37}
	fmt.Println("B is re-indexing:", flip(&pointer, b), " pointer", pointer)
	b.HeadLag, b.HealthyMin = 0, 12
	fmt.Println("B at the head:", flip(&pointer, b), " pointer", pointer)
	b.HealthyMin, b.GoldenPassed = 30, 37
	if err := flip(&pointer, b); err == nil {
		fmt.Println("B proven: the pull request merges, pointer", pointer)
	}
	pointer = "A" // git revert of the pointer commit
	fmt.Println("revert: pointer", pointer, " lane A never stopped")
	pointer = "B"

	la := map[string]string{"namespace": "lane-a", "host": "a.example", "image": "@cd34", "graphSyncMemory": "16Gi"}
	lb := map[string]string{"namespace": "lane-b", "host": "b.example", "image": "@cd34", "graphSyncMemory": "16Gi"}
	fmt.Println("same settings: drift", len(drift(la, lb)))
	lb["graphSyncMemory"] = "32Gi" // fixed on lane B only
	fmt.Println("fixed on B only: drift", drift(la, lb))
	la["graphSyncMemory"] = "32Gi" // the same session
	fmt.Println("fixed on both: drift", len(drift(la, lb)))
}
