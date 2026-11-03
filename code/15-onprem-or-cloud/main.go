// On-prem or cloud is a spreadsheet. All prices here are made-up examples
// to show the shape of the decision. They are not a quote.
package main

import (
	"fmt"
	"strings"
)

const hoursPerMonth = 730

type Load struct {
	Name        string
	BusyHours   float64 // per month
	CloudPerH   float64 // cloud price per busy hour
	EgressTB    float64 // data read out of the cloud, per month
	MetalFixed  float64 // metal price per month, busy or idle
	PeopleShare float64 // the share of the people who run the metal
}

const egressPerTB = 90 // cloud price per TB leaving the cloud

func cloudCost(l Load) float64 { return l.BusyHours*l.CloudPerH + l.EgressTB*egressPerTB }

func metalCost(l Load) float64 { return l.MetalFixed + l.PeopleShare }

// place picks the lower cost.
func place(l Load) string {
	c, m := cloudCost(l), metalCost(l)
	where := "cloud"
	if m < c {
		where = "metal"
	}
	fmt.Printf("  %-22s cloud %6.0f  metal %6.0f  -> %s\n", l.Name, c, m, where)
	return where
}

// spotHours adds up what a job pays for when spot nodes are taken back.
func spotHours(jobHours float64, reclaimAt []float64, checkpoint bool) float64 {
	done, paid := 0.0, 0.0
	for _, at := range reclaimAt {
		paid += at*jobHours - done
		if checkpoint {
			done = at * jobHours
		}
		if !checkpoint {
			done = 0
		}
	}
	return paid + jobHours - done
}

// exit keeps the choice open: what must stay portable.
var exit = []string{"Kubernetes manifests", "Parquet and Iceberg files", "no cloud-only service on the critical path"}

// pool picks where a task runs: spot while there is capacity, else the reserved fallback.
func pool(spotFree int) string {
	if spotFree == 0 {
		return "reserved fallback"
	}
	return "spot"
}

func main() {
	fmt.Println("Monthly, with example prices:")
	place(Load{"warehouse and graph", hoursPerMonth, 5, 0, 1200, 500})
	place(Load{"GPU training", 365, 6, 0, 1200, 300})
	place(Load{"website", 15, 2, 0, 400, 300})
	fmt.Println("Data gravity: warehouse in the cloud, training on metal reads 30 TB out:")
	fmt.Printf("  egress adds %.0f a month\n", 30.0*egressPerTB)

	const spot, reserved = 0.3, 0.6 // price per hour, examples
	without := spotHours(10, []float64{0.6, 0.5}, false)
	with := spotHours(10, []float64{0.6, 0.8}, true)
	fmt.Printf("Job of 10 h, no checkpoint, reclaimed at 60%% and 50%%: %.0f h on spot = %.1f\n", without, without*spot)
	fmt.Printf("The same job on a reserved node: 10 h = %.1f\n", 10*reserved)
	fmt.Printf("With checkpoints, reclaimed twice: %.0f h on spot = %.1f\n", with, with*spot)
	fmt.Println("No spot capacity left: the next task runs on", pool(0))
	fmt.Println("Exit kept open:", strings.Join(exit, ", "))
}
