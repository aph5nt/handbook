// The ingress decides where a request goes. A NetworkPolicy decides who may
// connect to a pod. A NodePort walks around both. Two retry layers multiply.
package main

import "fmt"

type Rule struct{ Host, Service string }

// route is the ingress controller's host rule lookup.
func route(rules []Rule, host string) string {
	for _, r := range rules {
		if r.Host == host {
			return r.Service
		}
	}
	return "404"
}

type Allow struct {
	FromNamespace string
	Port          int
}

// allowed is the NetworkPolicy check on a pod the policy selects.
func allowed(enforced bool, policy []Allow, fromNS string, port int) bool {
	if !enforced {
		return true // no enforcement: the policy is only a file
	}
	for _, a := range policy {
		if a.FromNamespace == fromNS && a.Port == port {
			return true
		}
	}
	return false
}

func show(ok bool) string {
	if ok {
		return "allowed"
	}
	return "dropped"
}

func main() {
	rules := []Rule{{"api.example", "api"}, {"grafana.example", "grafana"}}
	fmt.Println("api.example     ->", route(rules, "api.example"))
	fmt.Println("grafana.example ->", route(rules, "grafana.example"))
	fmt.Println("other.example   ->", route(rules, "other.example"))

	policy := []Allow{{"traefik", 8080}}
	for _, enforced := range []bool{false, true} {
		fmt.Printf("enforcement %v: ingress %s, Prometheus scrape %s\n", enforced,
			show(allowed(enforced, policy, "traefik", 8080)), show(allowed(enforced, policy, "monitoring", 9090)))
	}
	policy = append(policy, Allow{"monitoring", 9090})
	fmt.Printf("second rule added: ingress %s, Prometheus scrape %s\n",
		show(allowed(true, policy, "traefik", 8080)), show(allowed(true, policy, "monitoring", 9090)))

	fmt.Println("NodePort 30080 from the internet: reaches the pod, no TLS, no host rule, no rate limit")
	appTries, meshTries := 3, 3
	fmt.Printf("retries in the app and the mesh: %d x %d = %d database calls\n", appTries, meshTries, appTries*meshTries)
	fmt.Printf("retries in the mesh only: 1 x %d = %d database calls\n", meshTries, meshTries)
}
