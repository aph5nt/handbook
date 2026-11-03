// A private network: which connections can arrive. A target with no public
// address has no route in. A security group then checks source and port.
package main

import (
	"fmt"
	"net"
)

type Rule struct {
	FromGroup string // a security group, or "0.0.0.0/0" for anyone
	Port      int
}

type Machine struct {
	Name, Group, PrivateIP string
	Public                 bool
	Rules                  []Rule
}

// reach is the whole decision for one connection.
func reach(fromGroup string, fromOutside bool, m Machine, port int) string {
	if fromOutside && !m.Public {
		return "no route: no public address"
	}
	for _, r := range m.Rules {
		if (r.FromGroup == fromGroup || r.FromGroup == "0.0.0.0/0") && r.Port == port {
			return "allowed by " + r.FromGroup
		}
	}
	return "dropped by the security group"
}

// openToWorld is the alert: any rule open to anyone, except the edge.
func openToWorld(ms []Machine) []string {
	var out []string
	for _, m := range ms {
		for _, r := range m.Rules {
			if r.FromGroup == "0.0.0.0/0" && m.Name != "load balancer" {
				out = append(out, fmt.Sprintf("%s port %d", m.Name, r.Port))
			}
		}
	}
	return out
}

func main() {
	_, vpc, _ := net.ParseCIDR("10.0.0.0/16")
	for _, ip := range []string{"10.0.1.10", "10.0.2.20", "8.8.8.8"} {
		fmt.Printf("%s inside the VPC: %v\n", ip, vpc.Contains(net.ParseIP(ip)))
	}
	lb := Machine{"load balancer", "lb", "10.0.1.10", true, []Rule{{"0.0.0.0/0", 443}}}
	api := Machine{"API server", "api", "10.0.2.10", false, []Rule{{"lb", 8080}}}
	db := Machine{"database", "db", "10.0.2.20", false, []Rule{{"api", 5432}, {"bastion", 5432}}}

	fmt.Println("user -> load balancer :443:", reach("", true, lb, 443))
	fmt.Println("user -> API server :8080:", reach("", true, api, 8080))
	fmt.Println("load balancer -> API :8080:", reach("lb", false, api, 8080))
	fmt.Println("API -> database :5432:", reach("api", false, db, 5432))
	fmt.Println("load balancer -> database :5432:", reach("lb", false, db, 5432))
	fmt.Println("bastion -> database :5432:", reach("bastion", false, db, 5432))
	fmt.Println("scanner -> database :5432:", reach("", true, db, 5432))
	fmt.Println("alert:", openToWorld([]Machine{lb, api, db}))

	db.Public = true
	db.Rules = append(db.Rules, Rule{"0.0.0.0/0", 5432})
	fmt.Println("just this once, scanner -> database :5432:", reach("", true, db, 5432))
	fmt.Println("alert:", openToWorld([]Machine{lb, api, db}))
}
