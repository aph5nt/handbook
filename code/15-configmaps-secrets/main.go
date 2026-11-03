// ConfigMaps and Secrets: what a pod sees when a value changes, why a
// checksum annotation rolls the pods, and why a pushed password must be
// rotated.
package main

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

type Pod struct {
	Env      map[string]string // copied once, when the container starts
	Checksum string            // the pod template annotation it was made from
}

// checksum is what Helm's sha256sum puts in checksum/config.
func checksum(cm map[string]string) string {
	keys := []string{}
	for k := range cm {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		fmt.Fprintf(h, "%s=%s\n", k, cm[k])
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:8]
}

func start(cm map[string]string) Pod {
	env := map[string]string{}
	for k, v := range cm {
		env[k] = v
	}
	return Pod{Env: env, Checksum: checksum(cm)}
}

// reconcile is the Deployment controller: a new template checksum means a
// new ReplicaSet, so the pods are replaced and read the new values.
func reconcile(p Pod, cm map[string]string, withChecksum bool) Pod {
	if withChecksum && p.Checksum != checksum(cm) {
		fmt.Println("    template changed: roll the pods")
		return start(cm)
	}
	return p
}

// mayRead is the RBAC rule: one Role, one Secret name, one verb.
func mayRead(serviceAccount, secret string) bool {
	return serviceAccount == "indexer" && secret == "indexer-db"
}

func main() {
	cm := map[string]string{"NODE_URL": "http://node-2.local:8547", "PAGE_SIZE": "500"}
	secretFile := "s3cret-v1" // the mounted file, refreshed by the kubelet
	pod := start(cm)
	fmt.Println("start: PAGE_SIZE =", pod.Env["PAGE_SIZE"], " checksum", pod.Checksum)
	fmt.Println("RBAC: indexer may read indexer-db:", mayRead("indexer", "indexer-db"),
		"  default may read it:", mayRead("default", "indexer-db"))

	cm["PAGE_SIZE"] = "1000"
	for _, withChecksum := range []bool{false, true} {
		fmt.Printf("PAGE_SIZE set to 1000, checksum annotation %v:\n", withChecksum)
		p := reconcile(pod, cm, withChecksum)
		fmt.Println("    the pod uses PAGE_SIZE =", p.Env["PAGE_SIZE"])
	}

	history := []string{
		"a1 values-lane-b.yaml: dbPasswordRef: indexer-db",
		"b2 values-lane-b.yaml: dbPassword: s3cret-v1", // pushed in a hurry
		"c3 values-lane-b.yaml: dbPasswordRef: indexer-db (value removed)",
	}
	fmt.Println("git history after the delete:")
	for _, c := range history {
		leak := ""
		if strings.Contains(c, secretFile) {
			leak = "  <- still readable in every clone"
		}
		fmt.Println("   ", c+leak)
	}
	secretFile = "s3cret-v2" // rotated in the database and the store
	fmt.Println("after rotation the leaked value opens the database:", secretFile == "s3cret-v1")
}
