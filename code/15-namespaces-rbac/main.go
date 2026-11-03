// Namespaces, RBAC and quotas: the API server's two checks on every write.
//  1. Authorize: does a binding in this namespace give this subject a role
//     that allows this verb on this resource?
//  2. Admit: fill in defaults from the LimitRange, then refuse a pod that
//     would push the namespace past its ResourceQuota.
package main

import "fmt"

type Role map[string][]string // resource -> verbs

type Binding struct {
	Namespace string // "" for a ClusterRoleBinding: every namespace
	Subject   string
	Role      Role
}

var edit = Role{"deployments": {"get", "list", "update", "patch", "delete"}, "pods": {"get", "list", "create", "delete"}}
var view = Role{"deployments": {"get", "list"}, "pods": {"get", "list"}}
var clusterAdmin = Role{"*": {"*"}}

func allowed(bs []Binding, who, verb, resource, ns string) bool {
	for _, b := range bs {
		if b.Subject != who || (b.Namespace != "" && b.Namespace != ns) {
			continue
		}
		for _, r := range []string{resource, "*"} {
			for _, v := range b.Role[r] {
				if v == verb || v == "*" {
					return true
				}
			}
		}
	}
	return false // no binding allows it: 403 Forbidden
}

type Quota struct{ HardGi, UsedGi int }

type Pod struct{ RequestGi, LimitGi int }

// admit applies the LimitRange defaults and the ResourceQuota.
func admit(q *Quota, p Pod, n int) (Pod, error) {
	if p.RequestGi == 0 { // LimitRange: no pod is free
		p.RequestGi, p.LimitGi = 1, 2
	}
	ask := p.RequestGi * n
	if q.UsedGi+ask > q.HardGi {
		return p, fmt.Errorf("exceeded quota: asked %dGi, %dGi left", ask, q.HardGi-q.UsedGi)
	}
	q.UsedGi += ask
	return p, nil
}

func say(ok bool) string {
	if ok {
		return "allowed"
	}
	return "403 Forbidden"
}

func main() {
	bs := []Binding{{"lane-b", "agent", edit}}
	fmt.Println("agent patch lane-b/api:", say(allowed(bs, "agent", "patch", "deployments", "lane-b")))
	fmt.Println("agent patch lane-a/api:", say(allowed(bs, "agent", "patch", "deployments", "lane-a")))

	laneB := &Quota{HardGi: 900, UsedGi: 600}
	if _, err := admit(laneB, Pod{RequestGi: 40}, 10); err != nil {
		fmt.Println("staging test, 10 pods of 40Gi:", err)
	}
	p, _ := admit(laneB, Pod{}, 1)
	fmt.Printf("pod with no requests: filled in %dGi request, %dGi limit\n", p.RequestGi, p.LimitGi)

	shared := []Binding{{"", "engineer", clusterAdmin}}
	fmt.Println("everyone cluster-admin, delete lane-a/api:", say(allowed(shared, "engineer", "delete", "deployments", "lane-a")))
	least := []Binding{{"lane-a", "engineer", view}, {"lane-b", "engineer", edit}}
	fmt.Println("least privilege, delete lane-a/api:", say(allowed(least, "engineer", "delete", "deployments", "lane-a")))
	fmt.Println("least privilege, delete lane-b/api:", say(allowed(least, "engineer", "delete", "deployments", "lane-b")))
}
