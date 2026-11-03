// Probes: real /livez and /readyz handlers, and a kubelet on a one-second
// virtual clock. It replays the slow start with and without a startup
// probe, and a 5-second database pause with liveness on the process and
// liveness on the database.
package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
)

const (
	migrationSeconds = 60  // the check before the server can listen
	startupBudget    = 150 // the startup probe: 30 tries, every 5 s
	failureThreshold = 3   // liveness: three failures in a row
)

type DB struct{ Down bool }

func (d *DB) Ping() error {
	if d.Down {
		return errors.New("timeout")
	}
	return nil
}

func livez(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK) // no dependency: only "is this process stuck?"
}

func readyz(db *DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(); err != nil {
			http.Error(w, "database: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

type Pod struct {
	Age, Fails, Restarts int
	Started, Ready       bool
}

func (p *Pod) restart() {
	p.Age, p.Fails, p.Started, p.Ready = 0, 0, false, false
	p.Restarts++
}

type Kubelet struct {
	Mux          *http.ServeMux
	StartupProbe bool
	LivenessPath string
}

func (k *Kubelet) probe(p *Pod, path string) bool {
	if p.Age < migrationSeconds {
		return false // not listening yet: connection refused
	}
	rec := httptest.NewRecorder()
	k.Mux.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec.Code == http.StatusOK
}

// tick is one second of the kubelet for one pod.
func (k *Kubelet) tick(p *Pod) {
	p.Age++
	if k.StartupProbe && !p.Started {
		if k.probe(p, "/livez") {
			p.Started = true
		} else if p.Age > startupBudget {
			p.restart()
		}
		return // liveness and readiness wait for the startup probe
	}
	if k.probe(p, k.LivenessPath) {
		p.Fails = 0
	} else if p.Fails++; p.Fails >= failureThreshold {
		p.restart()
		return
	}
	p.Ready = k.probe(p, "/readyz") // the Service reads this
}

func setup() (*http.ServeMux, *DB) {
	db := &DB{}
	mux := http.NewServeMux()
	mux.HandleFunc("/livez", livez)
	mux.HandleFunc("/readyz", readyz(db))
	return mux, db
}

func start(startup bool) {
	mux, _ := setup()
	k := &Kubelet{Mux: mux, StartupProbe: startup, LivenessPath: "/livez"}
	p := &Pod{}
	for s := 1; s <= 300; s++ {
		k.tick(p)
		if p.Ready {
			fmt.Printf("  ready at %d s, restarts %d\n", s, p.Restarts)
			return
		}
	}
	fmt.Printf("  after 300 s: never ready, restarts %d\n", p.Restarts)
}

func hiccup(path string) {
	mux, db := setup()
	k := &Kubelet{Mux: mux, StartupProbe: true, LivenessPath: path}
	p := &Pod{}
	for s := 1; s <= 300; s++ {
		db.Down = s >= 100 && s < 105 // the 5-second pause
		k.tick(p)
		if s == 104 {
			fmt.Printf("  at 104 s: ready %v, restarts %d\n", p.Ready, p.Restarts)
		}
	}
	fmt.Printf("  at 300 s: ready %v, restarts %d\n", p.Ready, p.Restarts)
}

func main() {
	fmt.Println("Slow start, with a startup probe:")
	start(true)
	fmt.Println("Slow start, no startup probe:")
	start(false)
	fmt.Println("Database pause, liveness on /livez:")
	hiccup("/livez")
	fmt.Println("Database pause, liveness on /readyz (the database):")
	hiccup("/readyz")
}
