// A one-year export. Replays the lesson 13-long-running-operations: work
// inside the request is cut by the load balancer and retried into copies;
// 202 with a job resource makes the retry harmless. A short client timeout
// stands in for the load balancer's idle timeout.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---- held open: the work runs inside the request

type Held struct {
	mu      sync.Mutex
	started int
}

func (h *Held) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	h.started++
	h.mu.Unlock()
	time.Sleep(100 * time.Millisecond) // building the file, a heavy job
	fmt.Fprint(w, "file")
}

// ---- 202 and a job resource

type Job struct {
	ID       int    `json:"id"`
	State    string `json:"state"`
	Progress int    `json:"progress"`
	File     string `json:"file,omitempty"`
}

type API struct {
	jobs    map[int]*Job
	keys    map[string]int
	queue   []int
	webhook string
}

func (a *API) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /exports", a.create)
	mux.HandleFunc("GET /exports/{id}", a.poll)
	return mux
}

func (a *API) create(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("Idempotency-Key")
	id, seen := a.keys[key]
	if !seen {
		id = 91 + len(a.jobs)
		a.jobs[id] = &Job{ID: id, State: "queued"}
		a.keys[key] = id
		a.queue = append(a.queue, id)
	}
	w.Header().Set("Location", "/exports/"+strconv.Itoa(id))
	w.WriteHeader(http.StatusAccepted)
}

func (a *API) poll(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	job, ok := a.jobs[id]
	if !ok {
		http.Error(w, "no such job", http.StatusNotFound)
		return
	}
	if job.State != "done" {
		w.Header().Set("Retry-After", "10")
	}
	json.NewEncoder(w).Encode(job)
}

// work is the background worker: it moves one job forward.
func (a *API) work(id, progress int) {
	job := a.jobs[id]
	job.State, job.Progress = "running", progress
	if progress == 100 {
		job.State, job.File = "done", "/files/export-"+strconv.Itoa(id)+".csv"
		if a.webhook != "" {
			body, _ := json.Marshal(job)
			res, err := http.Post(a.webhook, "application/json", strings.NewReader(string(body)))
			if err == nil {
				res.Body.Close()
			}
		}
	}
}

func post(url, key string) *http.Response {
	req, _ := http.NewRequest("POST", url+"/exports", nil)
	req.Header.Set("Idempotency-Key", key)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	res.Body.Close()
	return res
}

func poll(url string) (Job, string) {
	res, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	var j Job
	json.NewDecoder(res.Body).Decode(&j)
	return j, res.Header.Get("Retry-After")
}

func main() {
	fmt.Println("--- request held open")
	h := &Held{}
	srv := httptest.NewServer(h)
	lb := &http.Client{Timeout: 20 * time.Millisecond} // the load balancer's cut
	for try := 1; try <= 3; try++ {
		_, err := lb.Post(srv.URL, "", nil)
		fmt.Println("try", try, "cut:", err != nil)
	}
	time.Sleep(150 * time.Millisecond)
	h.mu.Lock()
	fmt.Println("copies of the same export started:", h.started)
	h.mu.Unlock()
	srv.Close()

	fmt.Println("--- 202 and a job resource")
	a := &API{jobs: map[int]*Job{}, keys: map[string]int{}}
	api := httptest.NewServer(a.routes())
	defer api.Close()
	res := post(api.URL, "k7")
	loc := res.Header.Get("Location")
	fmt.Println("POST:", res.StatusCode, "Location", loc)
	a.work(a.queue[0], 40)
	j, after := poll(api.URL + loc)
	fmt.Println("poll:", j.State, j.Progress, "percent, Retry-After", after)
	res = post(api.URL, "k7")
	fmt.Println("same key again:", res.StatusCode, res.Header.Get("Location"), "jobs", len(a.jobs))
	got := make(chan string, 1)
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var j Job
		json.NewDecoder(r.Body).Decode(&j)
		got <- j.State
	}))
	defer hook.Close()
	a.webhook = hook.URL
	a.work(a.queue[0], 100)
	j, after = poll(api.URL + loc)
	fmt.Println("poll:", j.State, j.File, "Retry-After", after == "")
	fmt.Println("webhook said:", <-got)
}
