// Three ways to answer "may Ana read case 202?": a role, attributes, and a
// walk over relations. Each returns its reason, and the combined check logs.
package main

import "fmt"

type user struct {
	name, region string
	roles        []string
}
type caseFile struct{ id, region string }

var rolePerms = map[string][]string{"analyst": {"cases:read"}}

// rbac asks only: does a role hold this permission? It sees no case at all.
func rbac(u user, perm string) bool {
	for _, r := range u.roles {
		for _, p := range rolePerms[r] {
			if p == perm {
				return true
			}
		}
	}
	return false
}

// abac compares attributes of the user and of this case.
func abac(u user, c caseFile) bool { return u.region == c.region }

// relations are edges: subject, relation, object.
var relations = [][3]string{{"ana", "member", "team7"}, {"team7", "assigned", "case202"}, {"team2", "assigned", "case303"}}

// rebac walks the graph: user member of a team, team assigned this case.
func rebac(userName, caseID string) bool {
	for _, m := range relations {
		if m[0] == userName && m[1] == "member" {
			for _, a := range relations {
				if a[0] == m[2] && a[1] == "assigned" && a[2] == caseID {
					return true
				}
			}
		}
	}
	return false
}

// decide combines them: role for the coarse job, attribute for the region,
// relation for the exact case. It answers with its reason.
func decide(u user, c caseFile) (bool, string) {
	switch {
	case !rbac(u, "cases:read"):
		return false, "role does not allow cases:read"
	case !abac(u, c):
		return false, "region " + c.region + " is not " + u.region
	case !rebac(u.name, c.id):
		return false, "no assigned team for " + c.id
	}
	return true, "role analyst, region " + c.region + ", team assigned " + c.id
}

func main() {
	ana := user{"ana", "EU", []string{"analyst"}}
	c202, c303, c404 := caseFile{"case202", "EU"}, caseFile{"case303", "EU"}, caseFile{"case404", "US"}
	fmt.Println("  RBAC, any case                 ->", rbac(ana, "cases:read"))
	fmt.Println("  ABAC, case202 EU; case404 US   ->", abac(ana, c202), abac(ana, c404))
	fmt.Println("  ReBAC, case202; case303        ->", rebac("ana", "case202"), rebac("ana", "case303"))
	for _, c := range []caseFile{c202, c303, c404} {
		ok, why := decide(ana, c)
		fmt.Printf("  decide %-8s -> %-5v %s\n", c.id, ok, why)
	}
	regions, teams, access := 3, 500, 2 // analyst_eu_team7_ro and so on
	n := regions * teams * access
	fmt.Println("  RBAC only, role per region, team and access:", n, "roles")
}
