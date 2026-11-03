// A tiny terraform plan: compare the files with the state, apply moved
// blocks first, refuse a destroy of a protected resource, and hold a lock
// so two applies never write the state together.
package main

import (
	"errors"
	"fmt"
	"sort"
)

type Resource struct {
	Config         string
	PreventDestroy bool
}

type Plan struct{ Add, Change, Destroy []string }

// plan compares the wanted resources with the state.
func plan(files, state map[string]Resource, moved map[string]string) (Plan, error) {
	moves := map[string]Resource{}
	for name, r := range state { // moved blocks rename before the compare
		if to, ok := moved[name]; ok {
			name = to
		}
		moves[name] = r
	}
	var p Plan
	for name, want := range files {
		have, ok := moves[name]
		switch {
		case !ok:
			p.Add = append(p.Add, name)
		case have.Config != want.Config:
			p.Change = append(p.Change, name)
		}
	}
	for name, have := range moves {
		if _, ok := files[name]; !ok {
			if have.PreventDestroy {
				return p, fmt.Errorf("plan failed: %s has prevent_destroy", name)
			}
			p.Destroy = append(p.Destroy, name)
		}
	}
	sort.Strings(p.Add)
	sort.Strings(p.Destroy)
	return p, nil
}

func (p Plan) String() string {
	return fmt.Sprintf("%d to add, %d to change, %d to destroy", len(p.Add), len(p.Change), len(p.Destroy))
}

// Lock is the remote state lock.
type Lock struct{ holder string }

func (l *Lock) take(who string) error {
	if l.holder != "" {
		return errors.New("state locked by " + l.holder + ": " + who + " stops and waits")
	}
	l.holder = who
	return nil
}
func (l *Lock) release() { l.holder = "" }

func main() {
	state := map[string]Resource{
		"record.www": {Config: "old-host"},
		"db.main":    {Config: "large"},
	}
	files := map[string]Resource{
		"bucket.assets": {Config: "site-assets"},
		"record.www":    {Config: "new-host"},
		"db.main":       {Config: "large"},
	}
	p, _ := plan(files, state, nil)
	fmt.Println("the change:", p)
	state["bucket.assets"] = files["bucket.assets"]
	state["record.www"] = files["record.www"]

	renamed := map[string]Resource{
		"bucket.assets": files["bucket.assets"],
		"record.www":    files["record.www"],
		"db.main_db":    {Config: "large"},
	}
	p, _ = plan(renamed, state, nil)
	fmt.Println("rename main to main_db:", p, p.Destroy)
	p, _ = plan(renamed, state, map[string]string{"db.main": "db.main_db"})
	fmt.Println("with a moved block:", p)
	state["db.main"] = Resource{Config: "large", PreventDestroy: true}
	_, err := plan(renamed, state, nil)
	fmt.Println("with prevent_destroy, no moved block:", err)

	lock := &Lock{}
	lock.take("pipeline")
	fmt.Println(lock.take("laptop"))
	lock.release()
	fmt.Println("after release, laptop takes the lock:", lock.take("laptop") == nil)
}
