// Command uber replays the Uber lesson: a web of calls, domains with one
// gateway and one-way layers, then cells with a release wave and rollback,
// and the shared database that turns three cells back into one fleet.
package main

import "fmt"

type Service struct {
	Name, Domain string
	Gateway      bool
}

var layer = map[string]int{"Trips": 3, "Payments": 2, "Pricing": 1}

func Allowed(from, to Service, rules bool) string {
	if !rules {
		return "allowed: no rules"
	}
	if from.Domain == to.Domain {
		return "allowed: same domain"
	}
	if !to.Gateway {
		return "refused: use the gateway"
	}
	if layer[to.Domain] > layer[from.Domain] {
		return "refused: up is not allowed"
	}
	return "allowed: down, through the gateway"
}

func GatewayRoute(domain, op string) string {
	return map[string]string{"fare": "Fare", "surge": "Surge"}[op]
}

func Route(rider int) int { return rider%3 + 1 }

// Release deploys cell by cell and stops at the first bad cell.
func Release(errorRate map[int]float64) (done []int, rolledBack int) {
	for _, cell := range []int{2, 1, 3} {
		done = append(done, cell)
		if errorRate[cell] > 0.01 {
			return done[:len(done)-1], cell
		}
	}
	return done, 0
}

// RidersDown counts riders stopped by a lock in `lockedDB`.
func RidersDown(dbOf map[int]string, lockedDB string, ridersPerCell int) int {
	down := 0
	for _, db := range dbOf {
		if db == lockedDB {
			down += ridersPerCell
		}
	}
	return down
}

func main() {
	trip := Service{"Trip state", "Trips", false}
	fare := Service{"Fare", "Pricing", false}
	refund := Service{"Refund", "Payments", false}
	pricingGW := Service{"Pricing gateway", "Pricing", true}
	tripsGW := Service{"Trips gateway", "Trips", true}
	paymentsGW := Service{"Payments gateway", "Payments", true}
	fmt.Println("doma tab")
	fmt.Println("step 1-2: before, Refund -> Fare:", Allowed(refund, fare, false))
	fmt.Println("step 3: Refund -> Fare with rules:", Allowed(refund, fare, true))
	fmt.Println("step 4: Trip state -> Pricing gateway:", Allowed(trip, pricingGW, true), "-> inner", GatewayRoute("Pricing", "fare"))
	fmt.Println("step 5: Trip state -> Payments gateway:", Allowed(trip, paymentsGW, true))
	fmt.Println("step 5: Refund -> Trips gateway:", Allowed(refund, tripsGW, true))

	fmt.Println("cells tab")
	fmt.Printf("step 2: rider 3001 -> cell %d, rider 3002 -> cell %d, rider 3003 -> cell %d\n", Route(3001), Route(3002), Route(3003))
	done, back := Release(map[int]float64{2: 0.0})
	fmt.Println("step 3: good release: cells in order", done, "rollback", back)
	done, back = Release(map[int]float64{2: 1.0})
	fmt.Println("step 4-5: Pricing v2, cell 2 errors 100 % > 1 %: finished cells", done, "rolled back cell", back)
	own := map[int]string{1: "db1", 2: "db2", 3: "db3"}
	shared := map[int]string{1: "db", 2: "db", 3: "db"}
	fmt.Println("step 5: a lock in cell 2's own database stops riders:", RidersDown(own, "db2", 1000), "of 3000")
	fmt.Println("step 6: a lock in a shared database stops riders:", RidersDown(shared, "db", 1000), "of 3000")
}
