// The indexer twice: as a pet that keeps config, checkpoint and logs on
// its own disk, and as a twelve-factor process. Node 1 loses power after
// two pages, and a new pod starts on node 2.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

const pageSize = 200

// Store is the backing service: a checkpoint table in the database.
// It outlives every pod.
type Store struct{ next int }

func (s *Store) Next() int         { return s.next }
func (s *Store) SaveNext(next int) { s.next = next }

func indexPage(nodeURL string, from, to int) {} // upserts: safe to repeat

// run is the twelve-factor indexer. It stops after the page in hand when
// SIGTERM arrives. powerLoss > 0 kills it in the middle of that page.
func run(db *Store, maxPages, sigtermAfter, powerLoss int) {
	nodeURL := mustEnv("NODE_URL") // config from the environment
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{ReplaceAttr: noTime}))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer stop()
	next := db.Next() // resume, never restart
	log.Info("start", "node", nodeURL, "next", next)
	for p := 1; p <= maxPages; p++ {
		select {
		case <-ctx.Done(): // SIGTERM: the page in hand is already saved
			log.Info("SIGTERM, exiting", "next", next)
			return
		default:
		}
		last := next + pageSize - 1
		if p == powerLoss { // no SIGTERM, no warning, no save
			log.Info("power lost mid-page", "from", next, "saved next", db.Next())
			return
		}
		indexPage(nodeURL, next, last)
		db.SaveNext(last + 1) // after every page
		log.Info("page", "from", next, "to", last)
		next = last + 1
		if p == sigtermAfter { // a drain: the kubelet sends SIGTERM
			syscall.Kill(os.Getpid(), syscall.SIGTERM)
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func noTime(_ []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey {
		return slog.Attr{}
	}
	return a
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		fmt.Fprintln(os.Stderr, "missing", k) // fail fast, never a default
		os.Exit(1)
	}
	return v
}

// pet keeps config, checkpoint and log on the node's own disk (dir).
// A dead node takes all three with it.
func pet(dir string, pages int) {
	cfg, _ := os.ReadFile(filepath.Join(dir, "config.yaml")) // a hand edit lives here
	next := 1
	if b, err := os.ReadFile(filepath.Join(dir, "checkpoint.txt")); err == nil {
		fmt.Sscan(string(b), &next)
	} // no file on a new node: back to block 1
	logf, _ := os.OpenFile(filepath.Join(dir, "indexer.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	defer logf.Close()
	fmt.Printf("  pet starts: config %q, next block %d\n", cfg, next)
	for p := 0; p < pages; p++ {
		last := next + pageSize - 1
		indexPage(string(cfg), next, last)
		os.WriteFile(filepath.Join(dir, "checkpoint.txt"), []byte(fmt.Sprint(last+1)), 0o644)
		fmt.Fprintln(logf, "page", next, last) // only on this disk
		next = last + 1
	}
}

func main() {
	fmt.Println("Pet server:")
	node1, _ := os.MkdirTemp("", "node1-disk")
	node2, _ := os.MkdirTemp("", "node2-disk")
	defer os.RemoveAll(node1)
	defer os.RemoveAll(node2)
	os.WriteFile(filepath.Join(node1, "config.yaml"), []byte("node-2.local (edited by hand)"), 0o644)
	os.WriteFile(filepath.Join(node1, "checkpoint.txt"), []byte("9506000"), 0o644)
	pet(node1, 2)
	saved, _ := os.ReadFile(filepath.Join(node1, "checkpoint.txt"))
	fmt.Printf("  two pages done: checkpoint.txt on node 1 says %s\n", saved)
	fmt.Println("  03:00 node 1 loses power, its disk goes with it")
	os.WriteFile(filepath.Join(node2, "config.yaml"), []byte("node-1.local (the stale copy in the image)"), 0o644)
	pet(node2, 0) // no checkpoint on node 2's disk

	fmt.Println("Twelve-factor:")
	os.Setenv("NODE_URL", "http://node-2.local:8547") // from the ConfigMap
	db := &Store{next: 9506000}
	run(db, 5, 2, 0) // two pages, then a drain sends SIGTERM
	run(db, 5, 0, 1) // back on node 1, power lost in the first page
	fmt.Println("  new pod on node 2:")
	run(db, 1, 0, 0)
}
